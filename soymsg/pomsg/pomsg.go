// Package pomsg provides a PO file implementation for Soy message bundles
package pomsg

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/robfig/gettext/po"
	"github.com/robfig/soy/soymsg"
	"golang.org/x/text/language"
)

type provider struct {
	bundles map[string]soymsg.Bundle
}

// FileOpener defines an abstraction for opening a po file given a locale
type FileOpener interface {
	// Open returns ReadCloser for the po file indicated by locale. It returns
	// nil if the file does not exist
	Open(locale string) (io.ReadCloser, error)
}

// Load returns a soymsg.Provider that takes its translations by passing in the
// specified locales to the given PoFileProvider.
//
// Supports fallbacks for when a given locale does not exist, as long as the fallback files are in
// canonical form.
func Load(opener FileOpener, locales []string) (soymsg.Provider, error) {
	var prov = provider{make(map[string]soymsg.Bundle)}
	for _, locale := range locales {
		r, err := opener.Open(locale)
		if err != nil {
			return nil, err
		} else if r == nil {
			localeTag, err := language.Parse(locale)
			if err != nil {
				return nil, err
			}
			var r io.ReadCloser
			for _, fallbackLocale := range fallbacks(localeTag) {
				r, err = opener.Open(fallbackLocale.String())
				if err != nil {
					return nil, err
				}
				if r != nil {
					break
				}
			}
			if r == nil {
				continue
			}
		}


		pofile, err := po.Parse(r)
		r.Close()
		if err != nil {
			return nil, err
		}

		b, err := newBundle(locale, pofile)
		if err != nil {
			return nil, err
		}
		prov.bundles[locale] = b
	}
	return prov, nil
}

// fsFileOpener is a FileOpener based on the filesystem and rooted at Dirname
type fsFileOpener struct {
	Dirname string
}

func (o fsFileOpener) Open(locale string) (io.ReadCloser, error) {
	switch f, err := os.Open(path.Join(o.Dirname, locale+".po")); {
	case os.IsNotExist(err):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return f, nil
	}
}

// Dir returns a soymsg.Provider that takes translations from the given path.
// For example, if dir is "/usr/local/msgs", po files should be of the form:
//   /usr/local/msgs/<lang>.po
//   /usr/local/msgs/<lang>_<territory>.po
func Dir(dirname string) (soymsg.Provider, error) {
	var files, err = ioutil.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	var locales []string
	for _, fi := range files {
		var name = fi.Name()
		if !fi.IsDir() && strings.HasSuffix(name, ".po") {
			locales = append(locales, name[:len(name)-3])
		}
	}
	return Load(fsFileOpener{dirname}, locales)
}

func (p provider) Bundle(locale string) soymsg.Bundle {
	bundle, ok := p.bundles[locale]
	if !ok {
		tag, err := language.Parse(locale)
		if err != nil {
			return nil
		}
		for _, fb := range fallbacks(tag) {
			bundle, ok = p.bundles[fb.String()]
			if ok {
				break
			}
		}
	}
	return bundle
}

type bundle struct {
	messages  map[uint64]soymsg.Message
	locale    string
	pluralize po.PluralSelector
}

func newBundle(locale string, file po.File) (*bundle, error) {
	var pluralize = file.Pluralize
	if pluralize == nil {
		pluralize = po.PluralSelectorForLanguage(locale)
	}
	if pluralize == nil {
		return nil, fmt.Errorf("Plural-Forms must be specified")
	}

	var err error
	var msgs = make(map[uint64]soymsg.Message)
	for _, msg := range file.Messages {
		// Get the Message ID and plural var name
		var id uint64
		var varName string
		for _, ref := range msg.References {
			switch {
			case strings.HasPrefix(ref, "id="):
				id, err = strconv.ParseUint(ref[3:], 10, 64)
				if err != nil {
					return nil, err
				}
			case strings.HasPrefix(ref, "var="):
				varName = ref[len("var="):]
			}
		}
		if id == 0 {
			return nil, fmt.Errorf("no id found in message: %#v", msg)
		}
		msgs[id] = newMessage(id, varName, msg.Str)
	}
	return &bundle{msgs, locale, pluralize}, nil
}

func (b *bundle) Message(id uint64) *soymsg.Message {
	var msg, ok = b.messages[id]
	if !ok {
		return nil
	}
	return &msg
}

func (b *bundle) Locale() string {
	return b.locale
}

func (b *bundle) PluralCase(n int) int {
	return b.pluralize(n)
}

func newMessage(id uint64, varName string, msgstrs []string) soymsg.Message {
	if varName == "" && len(msgstrs) == 1 {
		return soymsg.Message{id, soymsg.Parts(msgstrs[0])}
	}

	var cases []soymsg.PluralCase
	for _, msgstr := range msgstrs {
		// TODO: Ideally this would convert from PO plural form to CLDR plural class.
		// Instead, just use PluralCase() to select one of these.
		cases = append(cases, soymsg.PluralCase{
			Spec:  soymsg.PluralSpec{soymsg.PluralSpecOther, -1}, // not used
			Parts: soymsg.Parts(msgstr),
		})
	}
	return soymsg.Message{id, []soymsg.Part{soymsg.PluralPart{
		VarName: varName,
		Cases:   cases,
	}}}
}
