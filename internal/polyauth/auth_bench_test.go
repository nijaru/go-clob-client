package polyauth

import "testing"

func BenchmarkHMACSignature(b *testing.B) {
	body := []byte(`{"hash":"0x123","owner":"0xabc","side":"BUY","price":"0.45","size":"10"}`)

	b.Run("double_quotes", func(b *testing.B) {
		for b.Loop() {
			if _, err := HMACSignature(
				"c2VjcmV0",
				1710000000,
				"POST",
				"/order",
				body,
			); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("single_quotes", func(b *testing.B) {
		body := []byte("{'hash':'0x123','owner':'0xabc','side':'BUY','price':'0.45','size':'10'}")
		for b.Loop() {
			if _, err := HMACSignature(
				"c2VjcmV0",
				1710000000,
				"POST",
				"/order",
				body,
			); err != nil {
				b.Fatal(err)
			}
		}
	})
}
