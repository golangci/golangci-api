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

	codecs := securecookie.CodecsFromPairs([]byte(os.Getenv("SESSION_SECRET")))
	if *encode {
		encodedValue, err := securecookie.EncodeMulti("s", *value, codecs...)
		if err != nil {
			panic(err)
		}

		log.Printf("encoded: %q", encodedValue)
	} else {
		var decodedValue string
		if err := securecookie.DecodeMulti("s", *value, &decodedValue, codecs...); err != nil {
			panic(err)
		}

		log.Printf("decoded: %q", decodedValue)
	}
}
