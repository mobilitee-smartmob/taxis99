package taxis99

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type mockServer struct {
	*httptest.Server

	Invoked bool
}

func newMockServer(hc *http.Client, handler func(w http.ResponseWriter, r *http.Request)) (*Client, *mockServer) {
	var mock mockServer
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mock.Invoked = true

		if handler != nil {
			handler(w, r)
		}
	}))
	mock.Server = s

	c := NewClient(hc)
	u, _ := url.Parse(s.URL + "/")
	c.BaseURL = u

	return c, &mock
}

func TestClientRequestMethod(t *testing.T) {
	testCases := []struct {
		method string
	}{
		{http.MethodGet},
		{http.MethodPost},
	}

	for _, tc := range testCases {
		t.Run(tc.method, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if got := r.Method; got != tc.method {
					t.Errorf("Got Request.Method %s; want %s.", got, tc.method)
				}
			}

			client, svr := newMockServer(nil, handler)
			defer svr.Close()

			err := client.Request(context.Background(), tc.method, "", nil, nil)
			if err != nil {
				t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
			}

			if !svr.Invoked {
				t.Error("Got server not called; want it to be called.")
			}
		})
	}
}

func TestClientRequestPath(t *testing.T) {
	testCases := []struct {
		path string
		want string
	}{
		{"employees", "/employees"},
		{"rides", "/rides"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Path; got != tc.want {
					t.Errorf("Got Request.URL.Path '%s'; want '%s'.", got, tc.want)
				}
			}

			client, srv := newMockServer(nil, handler)
			defer srv.Close()

			err := client.Request(context.Background(), http.MethodGet, tc.path, nil, nil)
			if err != nil {
				t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
			}

			if !srv.Invoked {
				t.Error("Got server not called; want it to be called.")
			}
		})
	}
}

func TestClientRequestHeader(t *testing.T) {
	testCases := []struct {
		body       interface{}
		wantHeader string
	}{
		{
			struct {
				Name string `json:"name"`
			}{"Test"},
			"application/json",
		},
		{
			nil,
			"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.wantHeader, func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				got := r.Header.Get("Content-Type")
				if got != tc.wantHeader {
					t.Errorf("Got Content-Type Header: '%s'; want '%s'.", got, tc.wantHeader)
				}
			}

			client, srv := newMockServer(nil, handler)
			defer srv.Close()

			err := client.Request(context.Background(), http.MethodGet, "", tc.body, nil)
			if err != nil {
				t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
			}

			if !srv.Invoked {
				t.Error("Got server not called; want it to be called.")
			}
		})
	}

}

func TestClientRequestBody(t *testing.T) {
	testCases := []struct {
		body     interface{}
		wantBody []byte
	}{
		{
			struct {
				Name string `json:"name"`
			}{"Test"},
			[]byte(`{"name":"Test"}`),
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.wantBody), func(t *testing.T) {
			handler := func(w http.ResponseWriter, r *http.Request) {
				got, err := ioutil.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("Got error while Request.Body: %s; want it to be nil.", err.Error())
				}
				if !bytes.Contains(got, tc.wantBody) {
					t.Errorf("got body: %s, want %s.", got, tc.wantBody)
				}
			}

			client, srv := newMockServer(nil, handler)
			defer srv.Close()

			err := client.Request(context.Background(), http.MethodGet, "", tc.body, nil)
			if err != nil {
				t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
			}

			if !srv.Invoked {
				t.Error("Got server not called; want it to be called.")
			}
		})
	}

}

func TestClientRequestResponseBody(t *testing.T) {
	type employee struct {
		Name string `json:"name"`
	}

	response := []byte(`{"name":"test"}`)

	t.Run(string(response), func(t *testing.T) {
		var emp employee
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.Write(response)
		}

		client, srv := newMockServer(nil, handler)

		err := client.Request(context.Background(), http.MethodGet, "", nil, &emp)
		if err != nil {
			t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
		}

		if !srv.Invoked {
			t.Error("Got server not called; want it to be called.")
		}

		if got, _ := json.Marshal(emp); !bytes.Contains(got, response) {
			t.Errorf("Got response '%s'; want '%s'.", got, response)
		}
	})
}

type testRoundTripperFn func(*http.Request) (*http.Response, error)

func (fn testRoundTripperFn) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestClientRequestEmptyOutput(t *testing.T) {
	response := []byte(`{"name":"test"}`)

	buf := bytes.NewBuffer(response)
	tripper := testRoundTripper(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		res := &http.Response{
			Body: ioutil.NopCloser(buf),
		}
		return res, nil
	})

	hc := &http.Client{
		Transport: tripper,
	}
	client := NewClient(hc)

	err := client.Request(context.Background(), http.MethodGet, "", nil, nil)
	if err != nil {
		t.Fatalf("Got error calling Request: %s; want it to be nil.", err.Error())
	}

	if buf.Len() == 0 {
		t.Error("Got Response.Body read; want it not to be read.")
	}
}

func TestClientRequestPathError(t *testing.T) {
	path := ":"
	c := NewClient(nil)

	err := c.Request(context.Background(), http.MethodGet, path, nil, nil)
	if err == nil {
		t.Error("Got error nil; want it not to be nil.")
	}
}

func TestClientRequestError(t *testing.T) {
	t.Run("NewRequest", func(t *testing.T) {
		c := NewClient(nil)
		err := c.Request(context.Background(), "ö", "", nil, nil)
		if err == nil {
			t.Error("Got error nil; want it not to be nil.")
		}
	})

	t.Run("Do", func(t *testing.T) {
		tripper := testRoundTripperFn(func(r *http.Request) (*http.Response, error) {
			defer r.Body.Close()
			return nil, errors.New("Testing error.")
		})
		hc := &http.Client{
			Transport: tripper,
		}

		c := NewClient(hc)

		err := c.Request(context.Background(), http.MethodGet, "", nil, nil)
		if err == nil {
			t.Error("Got error nil; want it not to be nil.")
		}
	})
}

func TestClienRequestErrorJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("nvalidJSON"))
	}

	client, srv := newMockServer(nil, handler)
	defer srv.Close()

	var out struct{}
	err := client.Request(context.Background(), http.MethodGet, "", nil, &out)
	if err == nil {
		t.Error("Got error nil; want it not to be nil.")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Error("Got error type not to be of APIError type; want it to be.")
	}
}

func TestClientRequestContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	//handler := func(w http.ResponseWriter, r *http.Request) {
	//	w.Write([]byte("nvalidJSON"))
	//}

	c, srv := newMockServer(nil, nil)
	defer srv.Close()

	err := c.Request(ctx, http.MethodGet, "", nil, nil)
	if err == nil {
		t.Error("Got error nil; want it not to be nil.")
	}
}