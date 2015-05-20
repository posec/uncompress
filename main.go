package main

import (
	"flag"
	"io"
	"log"
	"os"
)

func main() {
	flag.Parse()
	decompress(os.Stdin, os.Stdout)
}

var info = flag.Bool("info", false, "display various internal info")
var codeFlag = flag.Bool("code", false, "display all codes")

// Clear Code; used when block_mode is true.
const CLEAR = 256

func decompress(r io.Reader, w io.Writer) {
	header := make([]byte, 3)

	n, err := r.Read(header)
	if n < len(header) {
		log.Fatal("too short")
	}
	if err != nil {
		log.Fatal(err)
	}

	if header[0] != 037 || header[1] != 0235 {
		log.Fatal("not MAGIC")
	}
	maxbits := uint(header[2]) & 0x1f
	block_mode := (header[2] & 0x80) != 0
	maxmaxcode := 1 << maxbits

	if *info {
		log.Print("maxbits ", maxbits,
			" block mode ", block_mode)
	}
	if maxbits > 16 {
		log.Fatalf("compressed with %d bits; this is too much for me\n")
	}

	// The value of a suffix is a _byte_ (to output);
	// the value of a prefix is a _code_ (as int).
	suffixof := make([]byte, 256)
	prefixof := make([]int, 256)
	for i := uint(0); i < 256; i++ {
		suffixof[i] = byte(i)
		prefixof[i] = 99999
	}
	if block_mode {
		suffixof = append(suffixof, 0)
		prefixof = append(prefixof, 99999)
	}

	// Number of bits in input code, currently
	n_bits := uint(9)
	bitmask := 1<<n_bits - 1
	maxcode := bitmask
	var oldcode int
	// True only when the first code is being read
	first_code := true
	var finchar byte

	// Position, in bits, of next unread symbol.
	// Bits within a byte are indexed with 0 (mod 8) being
	// the least significant bit.
	posbits := uint(0)
	buf := []byte{}
	clear_flag := false

	for {
		if clear_flag ||
			posbits+n_bits > uint(len(buf))*8 ||
			len(prefixof) > maxcode {
			if len(prefixof) > maxcode {
				n_bits += 1
				bitmask = 1<<n_bits - 1
				if n_bits == maxbits {
					maxcode = maxmaxcode
				} else {
					maxcode = bitmask
				}
			}
			if clear_flag {
				n_bits = 9
				bitmask = 1<<n_bits - 1
				maxcode = bitmask
				clear_flag = false
			}
			buf = make([]byte, n_bits)
			n, err := r.Read(buf)
			if n == 0 {
				break
			}
			if err != nil && err != io.EOF {
				log.Println(err)
				break
			}
			buf = buf[:n]
			posbits = 0
		}

		// The next symbol is extracted from the next 2
		// or 3 bytes.
		i := posbits / 8
		e := (posbits + n_bits - 1) / 8
		l := int(buf[i]) + int(buf[i+1])<<8
		if e <= i {
			panic("e <= l")
		}
		if e > i+2 {
			panic("e > i+2")
		}
		if e == i+2 {
			l += int(buf[i+2]) << 16
		}
		code := (l >> (posbits % 8)) & bitmask
		posbits += n_bits
		if *codeFlag {
			log.Printf("[%d]", code)
		}

		if first_code {
			if code >= 256 {
				log.Fatalf("oldcode %v code %v\n", oldcode, code)
			}
			oldcode = code
			finchar = byte(code)
			w.Write([]byte{byte(code)})
			first_code = false
			continue
		}
		if code == CLEAR && block_mode {
			prefixof = prefixof[:256]
			suffixof = suffixof[:256]
			clear_flag = true
			continue
		}
		// The code from the input stream (code may
		// change later).
		incode := code

		stack := []byte{}

		if code >= len(prefixof) {
			// Special case for KwKwK
			if code > len(prefixof) {
				log.Fatalf("corrupt input, code=%v\n", code)
			}
			code = oldcode
			stack = []byte{finchar}
		}

		// Using the tables, reverse the code into a
		// sequence of bytes.
		for code >= 256 {
			stack = append([]byte{suffixof[code]}, stack...)
			code = prefixof[code]
		}

		finchar = suffixof[code]
		stack = append([]byte{finchar}, stack...)
		w.Write(stack)

		if len(prefixof) < maxmaxcode {
			prefixof = append(prefixof, oldcode)
			suffixof = append(suffixof, finchar)
		}
		oldcode = incode
	}
}
