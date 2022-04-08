package main

//Based off the awesome https://github.com/tomnomnom/unfurl

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	parser "github.com/Cgboal/DomainParser"
	uuid "github.com/satori/go.uuid"
	"net/http"
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

	var verbose, clean, unique, wayback bool
	flag.BoolVar(&unique, "unique", true, "")
	flag.BoolVar(&clean, "cleanurl", false, "")
	flag.BoolVar(&wayback, "waybackfetcher", true, "")
	flag.BoolVar(&verbose, "v", false, "")

	flag.Parse()

	//fmtStr := flag.Arg(1)

	seen := make(map[string]bool)

	sc := bufio.NewScanner(os.Stdin)

	for sc.Scan() {
		u, err := parseURL(sc.Text())
		if err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "parse failure: %s\n", err)
			}
			continue
		}
		if clean {
			endpointClean(u, unique, seen)
		} else if wayback {
			alive(u, unique, seen)
		}
	}

	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input: %s\n", err)
	}

}

// some urlProc functions return multiple things,
// so it's just easier to always get a slice and
// loop over it instead of having two kinds of
// urlProc functions.
func endpointClean(u *url.URL, unique bool, seen map[string]bool) {

	for _, val := range format(u, "%p") {

		// you do see empty values sometimes
		if val == "" {
			continue
		}

		if seen[val] && unique {
			continue
		}

		URLExtension := path.Ext(val)
		URLWithStrippedExtention := strings.TrimRight(val, URLExtension)

		if !uuidCheck(URLWithStrippedExtention) && !sha256Check(URLWithStrippedExtention) && !blacklistStringMatch(URLWithStrippedExtention) && !blacklistExtentionMatch(URLExtension) {
			fmt.Println(u)
		}

		// no point using up memory if we're outputting dupes
		if unique {
			seen[val] = true
		}
	}
}

func alive(u *url.URL, unique bool, seen map[string]bool) {

	for _, val := range format(u, "%p") {

		if seen[val] && unique {
			continue
		}

		resp, err := http.Get(u.String())
		if err != nil {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fmt.Print("https://web.archive.org/web/20060102150405if_/", u)
		}
	}
}

func blacklistExtentionMatch(URLExtension string) bool {

	blacklist := []string{"jpeg", "png", "svg", "jpg", "gif", "woff", "ttf", "scss"}

	for _, entry := range blacklist {
		if strings.Contains(URLExtension, entry) {
			return true
		}
	}
	return false
}

func blacklistStringMatch(fileWithStrippedExtention string) bool {

	blacklist := []string{"assets/frontend", "assets/static", "assets/vendor", "/fonts/"}

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
		return url.Parse(raw)
	}

	return u, nil
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
