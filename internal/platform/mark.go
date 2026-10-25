package platform

import "net/url"

// Windows' limit for an address (INTERNET_MAX_URL_LENGTH); longer ones aren't recorded anywhere.
const maxAddress = 2083

// webAddress returns an http(s) address as it is recorded on a downloaded file: without a login
// in it, and only if the file system's attribute can hold it. Anything else is "".
func webAddress(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	u.User = nil
	s := u.String()
	if len(s) > maxAddress {
		return ""
	}
	return s
}
