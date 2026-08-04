package dots

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// https://gosamples.dev/remove-non-printable-characters/
func filterNonPrintables(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, text)

	return text
}

func hasNonPrintable(text string) bool {
	pp := filterNonPrintables(text)
	return len(pp) != len(text)
}

func printable(suspects map[string]*string) error {
	// only white spaces or nothing
	pattern := "^\\s*$"
	re := regexp.MustCompile(pattern)

	for name, suspect := range suspects {
		if suspect == nil {
			continue
		}

		match := re.MatchString(*suspect)
		if match {
			return Errorf(EINVALID, fmt.Sprintf("%s is empty", name))
		}

		if has := hasNonPrintable(*suspect); has {
			return Errorf(EINVALID, fmt.Sprintf("%s is not a text line", name))
		}

		*suspect = strings.Trim(*suspect, " ")
	}

	return nil
}
