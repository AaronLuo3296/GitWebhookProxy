package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stakater/GitWebhookProxy/pkg/providers"
)

const (
	proxyGitlabTestSecret = "testSecret"
	proxyGitlabTestEvent  = "testEvent"
	proxyGitlabTestBody   = "testBody"
	httpBinURL            = "https://httpbin.org"
)

func TestProxy_isPathAllowed(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		path string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "isPathAllowedWithValidMultipleAllowedPaths",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithValidOneAllowedPaths",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1"},
				secret:       "secret",
			},
			args: args{
				path: "/path1",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithInvalidPath",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path3",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithEmtpyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndRootEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "/",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithAllPathsAllowedAndNonEmptyPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{},
				secret:       "secret",
			},
			args: args{
				path: "/path1",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSomePathsAllowedAndRootPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithSomePathsAllowedAndSubPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/path3",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithSubPathsAllowedAndSubPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/path3"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/path3",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSubPathsAllowedAndPathArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/path3"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: false,
		},
		{
			name: "isPathAllowedWithAllowedPathTrailingSlashAndNotInArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2/"},
				secret:       "secret",
			},
			args: args{
				path: "/path2",
			},
			want: true,
		},
		{
			name: "isPathAllowedWithSimpleAllowedPathAndTrailingSlashInArg",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  "https://dummyurl.com",
				allowedPaths: []string{"/path1", "/path2"},
				secret:       "secret",
			},
			args: args{
				path: "/path2/",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			if got := p.isPathAllowed(tt.args.path); got != tt.want {
				t.Errorf("Proxy.isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func createGitlabHook(tokenHeader string, tokenEvent string, body string) *providers.Hook {
	return &providers.Hook{
		Headers: map[string]string{
			providers.XGitlabToken: tokenHeader,
			providers.XGitlabEvent: tokenEvent,
		},
		Payload: []byte(body),
	}
}

func TestProxy_redirect(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		hook *providers.Hook
		path string
	}
	tests := []struct {
		name               string
		fields             fields
		args               args
		wantStatusCode     int
		wantRedirectedHost string // Only Host not complete URL
		wantErr            bool
	}{
		{
			name: "TestRedirectWithValidValues",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/post",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: "httpbin.org",
		},
		{
			name: "TestRedirectWithGetUpstream",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/get",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "TestRedirectWithEmptyPath",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURL + "/post",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: "httpbin.org",
		},
		{
			name: "TestRedirectWithEmptyPath",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURL + "/post",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: "httpbin.org",
		},
		{
			name: "TestRedirectWithNilHost",
			fields: fields{
				provider:     "github",
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/post",
				hook: nil,
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithInvalidUrl",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  "https://invalidurl",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/post",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithInvalidUrlScheme",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  "htttpsss://httpbin.org",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/post",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantErr: true,
		},
		{
			name: "TestRedirectWithUrlWithoutScheme",
			fields: fields{
				provider:     "gitlab",
				upstreamURL:  "httpbin.org",
				allowedPaths: []string{},
				secret:       "dummy",
			},
			args: args{
				path: "/post",
				hook: createGitlabHook(proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode:     http.StatusOK,
			wantRedirectedHost: "httpbin.org",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			gotResp, gotErrors := p.redirect(tt.args.hook, tt.args.path)

			if (gotErrors != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", gotErrors, tt.wantErr)
				return
			}

			if gotResp != nil && tt.wantErr == true {
				if gotResp.StatusCode != tt.wantStatusCode {
					t.Errorf("Proxy.redirect() got StatusCode in response= %v, want %v",
						gotResp.StatusCode, tt.wantStatusCode)
					return
				}

				if gotResp.Request.Host != tt.wantRedirectedHost {
					t.Errorf("Proxy.redirect() got Redirected Host in response= %v, want Redirected Host= %v",
						gotResp.Request.Host, tt.wantRedirectedHost)
					return
				}
			}
		})
	}
}

func createGitlabRequest(method string, path string, tokenHeader string,
	eventHeader string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Add(providers.XGitlabToken, tokenHeader)
	req.Header.Add(providers.XGitlabEvent, eventHeader)
	return req
}

func createRequestWithWrongHeadersKeys(method string, path string, tokenHeader string,
	eventHeader string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Add("X-Wrong-Token", tokenHeader)
	req.Header.Add("X-Wrong-Event", eventHeader)
	return req
}

func createRequestWithoutHeaders(method string, path string, body string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	return req
}

func TestProxy_proxyRequest(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		request *http.Request
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantStatusCode int
	}{
		{
			name: "TestProxyRequestWithValidValues",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithInvalidSecretInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					"InvalidSecret", proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptySecretInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					"", proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptyEventInHeader",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, "", proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithWrongHeaderKeys",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createRequestWithWrongHeadersKeys(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithoutHeaderKeys",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createRequestWithoutHeaders(http.MethodPost, "/post", proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithUnsupportedUrlPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/get",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "TestProxyRequestWithInvalidHttpMethod",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodGet, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestBody),
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name: "TestProxyRequestWithEmptyBody",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, ""),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithNotAllowedPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{"/path1"},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusForbidden,
		},
		{
			name: "TestProxyRequestWithAllowedPath",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{"/post"},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestProxyRequestWithInvalidUpstreamUrl",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  "invalidurl",
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "TestProxyRequestWithInvalidProvider",
			fields: fields{
				provider:     "invalid",
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
		{
			name: "TestProxyRequestWithWrongProviderKind",
			fields: fields{
				provider:     providers.GithubProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       proxyGitlabTestSecret,
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithInvalidSecretInProvider",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "wrong",
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusBadRequest,
		},
		{
			name: "TestProxyRequestWithEmptySecretInProvider",
			fields: fields{
				provider:     providers.GitlabProviderKind,
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				secret:       "",
			},
			args: args{
				request: createGitlabRequest(http.MethodPost, "/post",
					proxyGitlabTestSecret, proxyGitlabTestEvent, proxyGitlabTestSecret),
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			router := httprouter.New()
			router.POST("/*path", p.proxyRequest)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, tt.args.request)

			if status := rr.Code; status != tt.wantStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.wantStatusCode)
			}

		})
	}
}

func TestProxy_health(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		httpMethod string
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		wantStatusCode int
	}{
		{
			name: "TestHealthCheckGet",
			args: args{
				httpMethod: http.MethodGet,
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name: "TestHealthCheckPost",
			args: args{
				httpMethod: http.MethodPost,
			},
			wantStatusCode: http.StatusMethodNotAllowed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			router := httprouter.New()
			router.GET("/health", p.health)

			req, err := http.NewRequest(tt.args.httpMethod, "/health", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatusCode {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, tt.wantStatusCode)
			}
		})
	}
}

func TestProxy_Run(t *testing.T) {
	type fields struct {
		provider     string
		upstreamURL  string
		allowedPaths []string
		secret       string
	}
	type args struct {
		listenAddress string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		//https://stackoverflow.com/questions/46778600/golang-execute-function-after-http-listenandserve
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Proxy{
				provider:     tt.fields.provider,
				upstreamURL:  tt.fields.upstreamURL,
				allowedPaths: tt.fields.allowedPaths,
				secret:       tt.fields.secret,
			}
			if err := p.Run(tt.args.listenAddress); (err != nil) != tt.wantErr {
				t.Errorf("Proxy.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewProxy(t *testing.T) {
	type args struct {
		upstreamURL  string
		allowedPaths []string
		provider     string
		secret       string
	}
	tests := []struct {
		name    string
		args    args
		want    *Proxy
		wantErr bool
	}{
		{
			name: "TestNewProxyWithValidArgs",
			args: args{
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			want: &Proxy{
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
		},
		{
			name: "TestNewProxyWithEmptyUpstreamURL",
			args: args{
				upstreamURL:  "",
				allowedPaths: []string{},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithNilAllowedPaths",
			args: args{
				upstreamURL:  httpBinURL,
				allowedPaths: nil,
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithEmtpyProvider",
			args: args{
				upstreamURL:  httpBinURL,
				allowedPaths: []string{},
				provider:     "",
				secret:       proxyGitlabTestSecret,
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithEmtpySecret",
			args: args{
				upstreamURL:  httpBinURL,
				allowedPaths: nil,
				provider:     providers.GitlabProviderKind,
				secret:       "",
			},
			wantErr: true,
		},
		{
			name: "TestNewProxyWithValidArgsAndAllowedPaths",
			args: args{
				upstreamURL:  httpBinURL,
				allowedPaths: []string{"/path1", "/path2"},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
			want: &Proxy{
				upstreamURL:  httpBinURL,
				allowedPaths: []string{"/path1", "/path2"},
				provider:     providers.GitlabProviderKind,
				secret:       proxyGitlabTestSecret,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProxy(tt.args.upstreamURL, tt.args.allowedPaths, tt.args.provider, tt.args.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewProxy() = %v, want %v", got, tt.want)
			}
		})
	}
}
