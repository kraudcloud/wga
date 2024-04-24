package main

import (
	"bytes"
	"testing"
)

func TestEncrypt(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		input    string
		password string
	}{
		{
			name:     "Empty input",
			input:    "",
			password: "password",
		},
		{
			name:     "Single character",
			input:    "a",
			password: "password",
		},
		{
			name: "Basic wg config",
			input: `[Interface]
PrivateKey = MPeNZKVZYJ/AHqDctMbxb6exa5nGXI+F4iLYOkFwtVQ=
Address = 2a0e:6f07:8003:1:2::2/128

[Peer]
Endpoint = 185.112.181.161:51820
PublicKey = 2kYJdyRGVZOwOgJfNlFxjgqNpiI1KJx/Q50H2EIwRS0= 
AllowedIPs = 2a0e:6f07:8003:1:1::/80`,
			password: "Super$ecur3P@ssw0rd!",
		},
	}

	const maxURLSize = 2048

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := Encrypt(bytes.NewBufferString(tc.input), &buf, tc.password)
			if err != nil {
				t.Fatalf("Encrypt returned an error: %v", err)
			}

			var buf2 bytes.Buffer
			err = Decrypt(&buf, &buf2, tc.password)
			if err != nil {
				t.Fatalf("Decrypt returned an error: %v", err)
			}

			if buf.Len() > maxURLSize {
				t.Errorf("Encrypted string is too large: %d bytes", buf.Len())
			}

			if tc.input != buf2.String() {
				t.Errorf("Encrypt and Decrypt don't match: wanted %q, got %q", tc.input, buf2.String())
			}
		})
	}
}
