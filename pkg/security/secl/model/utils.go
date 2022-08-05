// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package model

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"regexp"
	"unicode"
	"unsafe"
)

// containerIDPattern is the pattern of a container ID
var containerIDPattern = regexp.MustCompile(fmt.Sprintf(`([[:xdigit:]]{%v})`, sha256.Size*2))

// FindContainerID extracts the first sub string that matches the pattern of a container ID
func FindContainerID(s string) string {
	return containerIDPattern.FindString(s)
}

// SliceToArray copy src bytes to dst. Destination should have enough space
func SliceToArray(src []byte, dst unsafe.Pointer) {
	for i := range src {
		*(*byte)(unsafe.Pointer(uintptr(dst) + uintptr(i))) = src[i]
	}
}

// UnmarshalStringArray extract array of string for array of byte
func UnmarshalStringArray(data []byte) ([]string, error) {
	var result []string
	len := uint32(len(data))

	for i := uint32(0); i < len; {
		if i+4 >= len {
			return result, ErrStringArrayOverflow
		}
		// size of arg
		n := ByteOrder.Uint32(data[i : i+4])
		if n == 0 {
			return result, nil
		}
		i += 4

		if i+n > len {
			// truncated
			arg := string(bytes.SplitN(data[i:len-1], []byte{0}, 2)[0])
			return append(result, arg), ErrStringArrayOverflow
		}

		arg := string(bytes.SplitN(data[i:i+n], []byte{0}, 2)[0])
		i += n

		result = append(result, arg)
	}

	return result, nil
}

// UnmarshalString unmarshal string
func UnmarshalString(data []byte, size int) (string, error) {
	if len(data) < size {
		return "", ErrNotEnoughData
	}

	i := bytes.IndexByte(data[:size], 0)
	if i < 0 {
		i = size
	}

	return string(data[:i]), nil
}

// UnmarshalPrintableString unmarshal printable string
func UnmarshalPrintableString(data []byte, size int) (string, error) {
	if len(data) < size {
		return "", ErrNotEnoughData
	}

	if len(data) < size {
		return "", ErrNotEnoughData
	}

	i := bytes.IndexFunc(data[:size], func(r rune) bool {
		return r == 0x00 || !unicode.IsOneOf(unicode.PrintRanges, r)
	})

	if i < 0 {
		i = size
	}

	if i == size || data[i] == 0x00 {
		return string(data[:i]), nil
	}

	return "", ErrNonPrintable
}
