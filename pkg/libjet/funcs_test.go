package libjet

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIndirect(t *testing.T) {
	sV := "test"
	sB := true
	sI := 12
	var pV *string

	type fooType struct {
		v int
	}
	var pF *fooType
	sF := fooType{}

	values := []struct {
		v      interface{}
		expect interface{}
		isNil  bool
	}{
		{sV, "test", false},
		{&sV, "test", false},
		{pV, nil, true},
		{sB, true, false},
		{&sB, true, false},
		{sI, 12, false},
		{&sI, 12, false},
		{fooType{}, fooType{}, false},
		{pF, nil, true},
		{sF, fooType{}, false},
		{fooType{2}, fooType{2}, false},
	}

	for _, tt := range values {
		r, isNil := Indirect(reflect.ValueOf(tt.v))
		assert.Exactly(t, tt.expect, r, "%#v", tt.v)
		assert.Equal(t, tt.isNil, isNil, "%#v", tt.v)
	}

}

func TestToString(t *testing.T) {
	sV := "test"
	sB := true
	sI := 12
	var pV *string

	type fooType struct {
		v int
	}
	var pF *fooType
	sF := fooType{}

	values := []struct {
		v      interface{}
		expect string
	}{
		{sV, "test"},
		{&sV, "test"},
		{pV, ""},
		{sB, "true"},
		{&sB, "true"},
		{sI, "12"},
		{&sI, "12"},
		{45.5, "45.5"},
		{fooType{}, "{0}"},
		{pF, ""},
		{sF, "{0}"},
		{fooType{2}, "{2}"},
		{[]byte("test"), "test"},
	}

	for _, tt := range values {
		v := ToString(reflect.ValueOf(tt.v))
		assert.Equal(t, tt.expect, v, "%#v", tt.v)
	}
}

func TestToDateFmt(t *testing.T) {
	var date1 *time.Time
	date2, _ := time.Parse("2006-01-02", "2012-04-02")
	date3 := &date2

	values := []struct {
		v      interface{}
		format string
		expect string
	}{
		{date1, "2006-01-02", ""},
		{date2, "2006-01-02", "2012-04-02"},
		{date3, "2006-01-02", "2012-04-02"},
	}

	for _, tt := range values {
		r := ToDateFmt(reflect.ValueOf(tt.v), reflect.ValueOf(tt.format))
		t.Logf("%#v\n", r)
		assert.Equal(t, tt.expect, r, "%#v", tt.v)
	}

	assert.Panics(t, func() {
		ToDateFmt(reflect.ValueOf(123), reflect.ValueOf(""))
	})
	assert.Panics(t, func() {
		v := &[]byte{}
		ToDateFmt(reflect.ValueOf(v), reflect.ValueOf(""))
	})
}
