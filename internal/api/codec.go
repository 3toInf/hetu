package api

import (
	"bufio"
	"encoding/json"
	"io"
)

func Encode(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func Decode(r io.Reader) (Request, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadBytes('\n')
	if err != nil {
		return Request{}, err
	}
	var req Request
	return req, json.Unmarshal(line, &req)
}
