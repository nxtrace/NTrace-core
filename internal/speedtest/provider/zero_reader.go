package provider

import "io"

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func ZeroBody(limit int64) io.Reader {
	if limit <= 0 {
		return io.LimitReader(zeroReader{}, 0)
	}
	return io.LimitReader(zeroReader{}, limit)
}
