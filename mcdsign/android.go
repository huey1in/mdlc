package mcdsign

// NewAndroid7036Signer is the APK 7.0.36.0 signing profile.
func NewAndroid7036Signer() *Signer {
	s, err := NewSigner(Credentials{
		AccessKey: "zWB5ZZDSaV5jjGoVz255",
		SecretKey: "Lch2Z94uveZQjGL6OMVUwqGNkEzZjfcVhKPwVHSu",
	})
	if err != nil {
		panic(err)
	}
	s.signedHeaders = []string{"ct", "language", "p", "sid", "sv", "token", "v", "x-mcd-gw-v"}
	return s
}
