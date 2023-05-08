package snclient

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
)

type ResponseWriterCapture struct {
	w          http.ResponseWriter
	body       bytes.Buffer
	statusCode int
}

func (i *ResponseWriterCapture) Write(buf []byte) (int, error) {
	_, err := i.body.Write(buf)
	LogError(err)

	n, err := i.w.Write(buf)
	if err != nil {
		return n, fmt.Errorf("response write failed: %s", err.Error())
	}

	return n, nil
}

func (i *ResponseWriterCapture) WriteHeader(statusCode int) {
	i.statusCode = statusCode
	i.w.WriteHeader(statusCode)
}

func (i ResponseWriterCapture) Header() http.Header {
	return i.w.Header()
}

func (i *ResponseWriterCapture) String(req *http.Request, body bool) string {
	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("HTTP/1.1 %d %s\n", i.statusCode, http.StatusText(i.statusCode)))
	for k, val := range i.w.Header() {
		for _, v := range val {
			buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
	}
	buf.WriteString("\n")
	buf.WriteString(i.body.String())

	reader := bufio.NewReader(strings.NewReader(buf.String()))
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		log.Errorf("response error: %s", err.Error())

		return ""
	}

	str, err := httputil.DumpResponse(resp, body)
	LogError(err)

	resp.Body.Close()

	return string(str)
}
