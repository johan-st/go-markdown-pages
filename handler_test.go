package main

import (
	"bytes"
	"io"
	"reflect"
	"testing"
)

func Test_preparePage(t *testing.T) {
	type args struct {
		src io.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    page
		wantErr bool
	}{
		{"empty", args{src: nil}, page{}, true},
		//	{"no header", args{src: bytes.NewBufferString("no header here")}, page{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := preparePage(tt.args.src)
			if (err != nil) != tt.wantErr {
				t.Errorf("preparePage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("preparePage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_xxx(t *testing.T) {
	b := bytes.NewBuffer([]byte("Hello, playground!\r\n.\r\nIrrelevant trailer."))
	c := make([]byte, 0, b.Len())
	for {
		p := b.Bytes()
		if bytes.Equal(p[:5], []byte("\r\n.\r\n")) {
			t.Logf("success: %s\n", string(c))
			t.FailNow()
			return
		}
		c = append(c, b.Next(1)...)
	}
}
