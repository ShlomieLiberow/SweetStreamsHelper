package main

//Based off the awesome https://github.com/tomnomnom/unfurl

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	parser "github.com/Cgboal/DomainParser"
	uuid "github.com/satori/go.uuid"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

var extractor parser.Parser

func init() {
	extractor = parser.NewDomainParser()
}

func main() {

	var unique bool
	flag.BoolVar(&unique, "u", true, "")

	var verbose bool
	flag.BoolVar(&verbose, "v", false, "")
	flag.BoolVar(&verbose, "verbose", false, "")

	flag.Parse()

	fmtStr := flag.Arg(1)

	sc := bufio.NewScanner(os.Stdin)

	seen := make(map[string]bool)

	for sc.Scan() {
		u, err := parseURL(sc.Text())
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "parse failure: %s\n", err)
			}
			continue
		}

		// some urlProc functions return multiple things,
		// so it's just easier to always get a slice and
		// loop over it instead of having two kinds of
		// urlProc functions.
		for _, val := range paths(u, fmtStr) {

			// you do see empty values sometimes
			if val == "" {
				continue
			}

			if seen[val] && unique {
				continue
			}

			fileExtension := path.Ext(val)
			fileWithStrippedExtention := strings.TrimRight(val, fileExtension)

			if !uuidCheck(fileWithStrippedExtention) && !sha256Check(fileWithStrippedExtention) && !stringMatch(fileWithStrippedExtention) {
				fmt.Println(u)
			}

			// no point using up memory if we're outputting dupes
			if unique {
				seen[val] = true
			}
		}
	}

	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}
}

func stringMatch(fileWithStrippedExtention string) bool {

	blacklist := []string{"assets/frontend", "assets/static", "assets/vendor"}

	for _, entry := range blacklist {
		if strings.Contains(fileWithStrippedExtention, entry) {
			return true
		}
	}
	return false
}

func uuidCheck(fileWithStrippedExtention string) bool {

	splitDir := strings.Split(fileWithStrippedExtention, "/")

	for _, dir := range splitDir {
		if len(dir) == 32 || len(dir) > 32 && len(dir) <= 60 && strings.Contains(dir, "-") {
			strip32Chars := fileWithStrippedExtention[len(fileWithStrippedExtention)-32:] //get last 32 chars to check is matches UUID format

			_, err := uuid.FromString(strip32Chars)
			if err == nil {
				return true
			}
		}
	}
	return false
}

func sha256Check(fileWithStrippedExtention string) bool {

	splitDir := strings.Split(fileWithStrippedExtention, "/")

	for _, dir := range splitDir {
		if len(dir) == 64 || len(dir) > 64 && strings.Contains(dir, "-") { //to filter example "constants-d28d254616822d54333b734a499081711780f22399c690af74070cf80d2007b4"
			strip32Chars := fileWithStrippedExtention[len(fileWithStrippedExtention)-32:] //get last 32 chars to check is matches UUID format

			_, err := uuid.FromString(strip32Chars)
			if err == nil {
				return true
			}
		}
	}
	return false
}

// parseURL parses a string as a URL and returns a *url.URL
// or any error that occured. If the initially parsed URL
// has no scheme, http:// is prepended and the string is
// re-parsed
func parseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	if u.Scheme == "" {
		return url.Parse("http://" + raw)
	}

	return u, nil
}

// paths returns the path portion of the URL. e.g.
// for http://sub.example.com/path it will return
// []string{"/path"}
func paths(u *url.URL, f string) []string {
	return format(u, "%p")
}

// format is a little bit like a special sprintf for
// URLs; it will return a single formatted string
// based on the URL and the format string. e.g. for
// http://example.com/path and format string "%d%p"
// it will return example.com/path
func format(u *url.URL, f string) []string {
	out := &bytes.Buffer{}

	inFormat := false
	for _, r := range f {

		if r == '%' && !inFormat {
			inFormat = true
			continue
		}

		if !inFormat {
			out.WriteRune(r)
			continue
		}

		switch r {

		// a literal percent rune
		case '%':
			out.WriteRune('%')

		// the scheme; e.g. http
		case 's':
			out.WriteString(u.Scheme)

		// the userinfo; e.g. user:pass
		case 'u':
			if u.User != nil {
				out.WriteString(u.User.String())
			}

		// the domain; e.g. sub.example.com
		case 'd':
			out.WriteString(u.Hostname())

		// the port; e.g. 8080
		case 'P':
			out.WriteString(u.Port())

		// the subdomain; e.g. www
		case 'S':
			out.WriteString(extractFromDomain(u, "subdomain"))

		// the root; e.g. example
		case 'r':
			out.WriteString(extractFromDomain(u, "root"))

		// the tld; e.g. com
		case 't':
			out.WriteString(extractFromDomain(u, "tld"))

		// the path; e.g. /users
		case 'p':
			out.WriteString(u.EscapedPath())

		// the paths's file extension
		case 'e':
			parts := strings.Split(u.EscapedPath(), ".")
			if len(parts) > 1 {
				out.WriteString(parts[len(parts)-1])
			}

		// the query string; e.g. one=1&two=2
		case 'q':
			out.WriteString(u.RawQuery)

		// the fragment / hash value; e.g. section-1
		case 'f':
			out.WriteString(u.Fragment)

		// an @ if user info is specified
		case '@':
			if u.User != nil {
				out.WriteRune('@')
			}

		// a colon if a port is specified
		case ':':
			if u.Port() != "" {
				out.WriteRune(':')
			}

		// a question mark if there's a query string
		case '?':
			if u.RawQuery != "" {
				out.WriteRune('?')
			}

		// a hash if there is a fragment
		case '#':
			if u.Fragment != "" {
				out.WriteRune('#')
			}

		// the authority; e.g. user:pass@example.com:8080
		case 'a':
			out.WriteString(format(u, "%u%@%d%:%P")[0])

		// default to literal
		default:
			// output untouched
			out.WriteRune('%')
			out.WriteRune(r)
		}

		inFormat = false
	}

	return []string{out.String()}
}

func extractFromDomain(u *url.URL, selection string) string {

	// remove the port before parsing
	portRe := regexp.MustCompile(`(?m):\d+$`)

	domain := portRe.ReplaceAllString(u.Host, "")

	switch selection {
	case "subdomain":
		return extractor.GetSubdomain(domain)
	case "root":
		return extractor.GetDomain(domain)
	case "tld":
		return extractor.GetTld(domain)
	default:
		return ""
	}
}
