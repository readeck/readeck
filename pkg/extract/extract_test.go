package extract

import (
	"errors"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	errlist := Error{errors.New("err1"), errors.New("err2")}
	assert.Equal(t, "err1, err2", errlist.Error())
}

func TestURLList(t *testing.T) {
	list := URLList{}
	list.Add(mustParse("http://example.net/main"))

	assert.True(t, list.IsPresent(mustParse("http://example.net/main")))
	assert.False(t, list.IsPresent(mustParse("http://example.org/")))

	list.Add(mustParse("http://example.org/"))
	assert.True(t, list.IsPresent(mustParse("http://example.org/")))
}

func TestExtractor(t *testing.T) {
	t.Run("new with error", func(t *testing.T) {
		ex, err := New("http://example.net/\b0x7f", nil)
		assert.Nil(t, ex)
		assert.Contains(t, err.Error(), "invalid control")
	})

	t.Run("new", func(t *testing.T) {
		ex, _ := New("http://example.net/#frag", nil)
		assert.Equal(t, "http://example.net/", ex.URL.String())
		assert.Equal(t, 1, len(ex.Drops()))

		drop := ex.Drop()
		assert.Equal(t, drop, ex.Drops()[0])
		assert.Equal(t, "http://example.net/", drop.URL.String())

		assert.IsType(t, NewClient(), ex.Client())
		assert.Equal(t, 0, len(ex.Errors()))

		ex.AddError(errors.New("err1"))
		assert.Equal(t, "err1", ex.Errors().Error())
	})

	t.Run("drops", func(t *testing.T) {
		ex := Extractor{}
		assert.Nil(t, ex.Drop())

		ex.AddDrop(mustParse("http://example.net/"))
		assert.Equal(t, "http://example.net/", ex.Drop().URL.String())

		ex.ReplaceDrop(mustParse("http://example.net/new"))
		assert.Equal(t, "http://example.net/new", ex.Drop().URL.String())

		ex.AddDrop(mustParse("http://example.net/page2"))
		err := ex.ReplaceDrop(mustParse("http://example.net/page1"))
		assert.Equal(t,
			"cannot replace a drop when there are more that one",
			err.Error())
	})
}

func TestExtractorRun(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/page1", newHTMLResponder(200, "html/ex1.html"))

	p1 := func(m *ProcessMessage, next Processor) Processor {
		if m.Step() != StepBody {
			return next
		}

		m.Extractor.Drops()[m.Position].Body = []byte("test")
		return next
	}

	p2a := func(m *ProcessMessage, next Processor) Processor {
		if m.Step() != StepBody {
			return next
		}

		m.SetValue("newbody", []byte("@@body@@"))

		return next
	}

	p2b := func(m *ProcessMessage, next Processor) Processor {
		if m.Step() != StepBody {
			return next
		}

		m.Extractor.Drops()[m.Position].Body = m.Value("newbody").([]byte)

		return next
	}

	p3 := func(m *ProcessMessage, next Processor) Processor {
		if m.Step() != StepBody {
			return next
		}

		if m.Position == 0 {
			m.Extractor.AddDrop(mustParse("http://example.org/page1"))
		}
		if m.Position == 1 {
			m.Extractor.AddDrop(mustParse("http://example.net/page1"))
		}
		if m.Position > 2 {
			// That will never happen
			panic("We should never loop")
		}
		return next
	}

	t.Run("simple", func(t *testing.T) {
		ex, _ := New("http://example.net/page1", nil)
		ex.Run()
		assert.Equal(t, 0, len(ex.Errors()))
		assert.Contains(t, string(ex.Drop().Body), "Otters have long, slim bodies")
	})

	t.Run("load error", func(t *testing.T) {
		ex, _ := New("http://example.net/404", nil)
		ex.Run()
		assert.Equal(t, 1, len(ex.Errors()))
		assert.Equal(t, "cannot load resource", ex.Errors().Error())
	})

	t.Run("process body", func(t *testing.T) {
		ex, _ := New("http://example.net/page1", nil)
		ex.AddProcessors(p1)
		ex.Run()
		assert.Equal(t, 0, len(ex.Errors()))
		assert.Equal(t,
			"test",
			string(ex.Drop().Body))
	})

	t.Run("process passing values", func(t *testing.T) {
		ex, _ := New("http://example.net/page1", nil)
		ex.AddProcessors(p2a, p2b)
		ex.Run()
		assert.Equal(t, 0, len(ex.Errors()))
		assert.Equal(t,
			"@@body@@",
			string(ex.Drop().Body))
	})

	t.Run("process add drop", func(t *testing.T) {
		ex, _ := New("http://example.net/page1", nil)
		ex.AddProcessors(p3)
		ex.Run()
		assert.Equal(t, 0, len(ex.Errors()))
		assert.Equal(t, 3, len(ex.Drops()))
		assert.Equal(t, "http://example.net/page1", ex.Drops()[0].URL.String())
		assert.Equal(t, "http://example.org/page1", ex.Drops()[1].URL.String())
	})

}
