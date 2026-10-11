// Package mcdsign reproduces the HMAC v4 request signing performed by the
// McDonald's China Android APK. It creates signing headers and random logical
// device IDs, and never sends a network request. Callers should persist a device
// ID per account and reuse it rather than generating one for each request.
package mcdsign
