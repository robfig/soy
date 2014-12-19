// Package pomsg provides a PO file implementation for Soy message bundles
package pomsg

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/robfig/gettext/po"
	"github.com/robfig/soy/soymsg"
)

type provider struct {
	bundles map[string]soymsg.Bundle
}

// Dir returns a soymsg.Bundle that takes translations from the given path.
// For example, if dir is "/usr/local/msgs", po files should be of the form:
//   /usr/local/msgs/<lang>.po
//   /usr/local/msgs/<lang>_<territory>.po
//
// TODO: Fallbacks between <lang> and <lang>_<territory>
func Dir(dirname string) (soymsg.Provider, error) {
	var files, err = ioutil.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	var prov = provider{make(map[string]soymsg.Bundle)}
	for _, fi := range files {
		var name = fi.Name()
		if !fi.IsDir() && strings.HasSuffix(name, ".po") {
			var f, err = os.Open(path.Join(dirname, name))
			if err != nil {
				return nil, err
			}
			pofile, err := po.Parse(f)
			if err != nil {
				return nil, err
			}
			var locale = name[:len(name)-3]
			b, err := newBundle(locale, pofile)
			if err != nil {
				return nil, err
			}
			prov.bundles[locale] = b
		}
	}
	return prov, nil
}

func (p provider) Bundle(locale string) soymsg.Bundle {
	return p.bundles[locale]
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
