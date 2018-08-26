package main

import (
	"flag"
	"log"
	"os"

	"github.com/gorilla/securecookie"
)

func main() {
	value := flag.String("value", "", "cookie/session value")
	encode := flag.Bool("encode", true, "Encode/Decode")
	flag.Parse()

	sessSecret := os.Getenv("SESSION_SECRET")
	if sessSecret == "" {
		panic("SESSION_SECRET isn't set")
	}

	codecs := securecookie.CodecsFromPairs([]byte(sessSecret))
	if *encode {
		encodedValue, err := securecookie.EncodeMulti("s", *value, codecs...)
		if err != nil {
			log.Fatalf("Can't encode: %s", err)
		}

		log.Printf("Encoded: %q", encodedValue)
	} else {
		var decodedValue string
		if err := securecookie.DecodeMulti("s", *value, &decodedValue, codecs...); err != nil {
			log.Fatalf("Can't decode %q: %s", *value, err)
		}

		log.Printf("Decoded: %q", decodedValue)
	}
}
