package utils

import (
	"encoding/base64"
	"regexp"
)

const (
	Base64Token = "base64"
)

// ExtractFromComplexValue does extract value from following structure base64(<encoded value>) and is decoding the value itself
func ExtractFromComplexValue(value string) (string, error) {
	re := regexp.MustCompile(`^(\w+)\((.*)\)$`)

	for {
		matches := re.FindStringSubmatch(value)
		if len(matches) < 3 {
			return value, nil
		}

		outer := matches[1]
		payload := matches[2]

		switch outer {
		case Base64Token:
			decoded, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return "", err
			}

			value = string(decoded)
		default:
			return value, nil
		}
	}
}
