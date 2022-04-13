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
	"sort"
	"strings"
)

var extractor parser.Parser

func init() {
	extractor = parser.NewDomainParser()
}

func main() {

	var verbose, clean, unique, wayback bool
	flag.BoolVar(&unique, "unique", true, "")
	flag.BoolVar(&clean, "clean", false, "")
	flag.BoolVar(&wayback, "wbfetcher", false, "")
	flag.BoolVar(&verbose, "v", false, "")
	flag.Parse()

	if !clean && !wayback {
		fmt.Fprintf(os.Stderr, "no mode selected")
		return
	}

	// Check for stdin input
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		fmt.Fprintln(os.Stderr, "No input detected")
		//os.Exit(1)
	}

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
			waybackForDeadEndpoints(u, unique, seen)
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

	for _, pathStr := range format(u, "%p") {

		// you do see empty values sometimes
		if pathStr == "" {
			continue
		}

		paramKey := strings.Join(keys(u), "&")

		if seen[pathStr+paramKey] && unique {
			continue
		}

		URLExtension := path.Ext(pathStr)
		URLWithStrippedExtention := strings.TrimRight(pathStr, URLExtension)

		if isUUID(URLWithStrippedExtention) || isSHA256(URLWithStrippedExtention) || blacklistStringMatch(URLWithStrippedExtention) || blacklistExtentionMatch(URLExtension) {
			continue
		}
		fmt.Println(regexClean(u.String()))

		if unique {
			seen[pathStr+paramKey] = true
		}
	}
}

func regexClean(stringToClean string) string {

	listOfPatterns := []string{`\?v=.*?$`, `#.*?$`} // removeVersions = `\?v=.*?$` removehashes = `#.*?$`

	for _, listOfPatterns := range listOfPatterns {
		var myPointer = &stringToClean

		regExp, err := regexp.Compile(listOfPatterns)
		if err != nil {
			return ""
		}

		*myPointer = regExp.ReplaceAllString(stringToClean, "")
	}

	return stringToClean

}

func waybackForDeadEndpoints(u *url.URL, unique bool, seen map[string]bool) {

	for _, val := range format(u, "%p") {

		if seen[val] && unique {
			continue
		}

		resp, err := http.Head(u.String())
		if err != nil {
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			fmt.Print("https://web.archive.org/web/20060102150405if_/", u, "\n")
			//TODO pipe into nuclei?
		}
	}
}

func blacklistExtentionMatch(URLExtension string) bool {

	blacklist := []string{"jpeg", "png", "svg", "jpg", "ico", "swf", "gif", "woff", "ttf", "scss", "css", ".eot"}

	for _, entry := range blacklist {
		if strings.Contains(URLExtension, entry) {
			fmt.Println("dddd")
			return true
		}
	}
	return false
}

func blacklistStringMatch(fileWithStrippedExtention string) bool {

	blacklist := []string{"assets/frontend", "assets/static", "assets/vendor", "/fonts/", "article/", "/blog/"}

	for _, entry := range blacklist {
		if strings.Contains(fileWithStrippedExtention, entry) {
			fmt.Println("rrrr")
			return true
		}
	}
	return false
}

func isUUID(fileWithStrippedExtention string) bool {

	splitDir := strings.Split(fileWithStrippedExtention, "/")

	for _, dir := range splitDir {
		if len(dir) == 32 || len(dir) == 36 { //d80c0a07-a503-4c97-b996-1441d827dab5 or d80c0a07a5034c97b9961441d827dab5 TODO Bother with blbl-d80c0a07a5034c97b9961441d827dab5?
			_, err := uuid.FromString(dir)
			if err == nil {
				return true
			}
		}
	}
	return false
}

func isSHA256(fileWithStrippedExtention string) bool {

	splitDir := strings.Split(fileWithStrippedExtention, "/")

	for _, dir := range splitDir {
		if len(dir) == 64 || len(dir) > 64 && dir[len(dir)-65:len(dir)-64] == "-" { //to filter example "constants-d28d254616822d54333b734a499081711780f22399c690af74070cf80d2007b4"
			return true
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

// keys returns all of the keys used in the query string
// portion of the URL. E.g. for /?one=1&two=2&three=3 it
// will return []string{"one", "two", "three"}
func keys(u *url.URL) []string {
	out := make([]string, 0)
	for key := range u.Query() {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
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
