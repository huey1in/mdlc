//go:build linux

// dexdump reads DEX images from a local Android test process.
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	pkg := flag.String("package", "com.mcdonalds.gma.cn", "exact process name")
	out := flag.String("out", "/data/local/tmp/dex-dump", "output directory")
	minRSS := flag.Int("min-rss", 280000, "minimum RSS in KiB before attaching")
	timeout := flag.Duration("timeout", 20*time.Second, "process discovery timeout")
	flag.Parse()
	runtime.LockOSThread() // ptrace operations must stay on the attaching OS thread.
	if err := run(*pkg, *out, *minRSS, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(pkg, out string, minRSS int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	fmt.Println("waiting for", pkg)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir("/proc")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			pid, err := strconv.Atoi(entry.Name())
			if err != nil {
				continue
			}
			base := filepath.Join("/proc", entry.Name())
			cmd, _ := os.ReadFile(base + "/cmdline")
			if strings.SplitN(string(cmd), "\x00", 2)[0] != pkg {
				continue
			}
			status, _ := os.ReadFile(base + "/status")
			rss := 0
			for _, line := range strings.Split(string(status), "\n") {
				if strings.HasPrefix(line, "VmRSS:") {
					fmt.Sscanf(line, "VmRSS: %d", &rss)
				}
			}
			if rss < minRSS {
				continue
			}
			fmt.Printf("attaching pid=%d rss=%d KiB\n", pid, rss)
			time.Sleep(50 * time.Millisecond)
			return dump(pid, out)
		}
		time.Sleep(2 * time.Millisecond)
	}
	return fmt.Errorf("no matching process reached minimum RSS before timeout")
}

func dump(pid int, out string) error {
	if err := syscall.PtraceAttach(pid); err != nil {
		return fmt.Errorf("ptrace attach: %w", err)
	}
	defer syscall.PtraceDetach(pid)
	var status syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &status, 0, nil); err != nil {
		return err
	}
	if !status.Stopped() {
		return fmt.Errorf("process did not stop: %v", status)
	}
	mem, err := os.Open(fmt.Sprintf("/proc/%d/mem", pid))
	if err != nil {
		return err
	}
	defer mem.Close()
	maps, err := os.Open(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return err
	}
	defer maps.Close()
	if err := os.MkdirAll(out, 0700); err != nil {
		return err
	}
	mapBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "maps.txt"), mapBytes, 0600); err != nil {
		return err
	}
	seen := map[uint64]bool{}
	count := 0
	scanner := bufio.NewScanner(maps)
	buffer := make([]byte, 1024*1024+0x70)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "r") {
			continue
		}
		var start, end uint64
		if _, err := fmt.Sscanf(fields[0], "%x-%x", &start, &end); err != nil {
			continue
		}
		for pos := start; pos < end; pos += 1024 * 1024 {
			length := uint64(len(buffer))
			if end-pos < length {
				length = end - pos
			}
			n, _ := mem.ReadAt(buffer[:int(length)], int64(pos))
			data := buffer[:n]
			for offset := 0; offset+0x70 <= len(data); {
				i := bytes.Index(data[offset:], []byte("dex\n"))
				if i < 0 {
					break
				}
				i += offset
				offset = i + 4
				if i+0x70 > len(data) {
					break
				}
				h := data[i : i+0x70]
				address := pos + uint64(i)
				if seen[address] || h[7] != 0 || h[4] != '0' || h[5] != '3' || h[6] < '5' || h[6] > '9' {
					continue
				}
				size := binary.LittleEndian.Uint32(h[0x20:])
				if binary.LittleEndian.Uint32(h[0x24:]) != 0x70 || binary.LittleEndian.Uint32(h[0x28:]) != 0x12345678 || size < 0x70 || size > 256*1024*1024 {
					continue
				}
				seen[address] = true
				// SecNeo may place method bodies after the original file_size.
				// Preserve the rest of this mapping so relocated offsets remain readable.
				if tail := end - address; tail > uint64(size) && tail <= 256*1024*1024 {
					size = uint32(tail)
				}
				name := filepath.Join(out, fmt.Sprintf("dex-%x-%d.dex", address, size))
				f, err := os.OpenFile(name+".partial", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				written, copyErr := io.CopyN(f, io.NewSectionReader(mem, int64(address), int64(size)), int64(size))
				closeErr := f.Close()
				if copyErr != nil || closeErr != nil {
					fmt.Printf("incomplete address=%x bytes=%d error=%v close=%v\n", address, written, copyErr, closeErr)
					continue
				}
				if err := os.Rename(name+".partial", name); err != nil {
					return err
				}
				count++
				fmt.Printf("dumped %s\n", name)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	fmt.Printf("complete: %d DEX images\n", count)
	if count == 0 {
		return fmt.Errorf("no complete DEX images found")
	}
	return nil
}
