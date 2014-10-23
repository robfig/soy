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
		if !fi.IsDir() && strings.HasSuffix(fi.Name(), ".po") {
			var f, err = os.Open(path.Join(dirname, fi.Name()))
			if err != nil {
				return nil, err
			}
			pofile, err := po.Parse(f)
			if err != nil {
				return nil, err
			}
			b, err := newBundle(pofile)
			if err != nil {
				return nil, err
			}
			prov.bundles[fi.Name()[:len(fi.Name())-3]] = b
		}
	}
	return prov, nil
}

func (p provider) Bundle(locale string) soymsg.Bundle {
	return p.bundles[locale]
}

type bundle struct {
	messages map[uint64]soymsg.Message
}

func newBundle(file po.File) (*bundle, error) {
	var err error
	var msgs = make(map[uint64]soymsg.Message)
	for _, msg := range file.Messages {
		// Get the Message ID
		var id uint64
		for _, ref := range msg.References {
			if strings.HasPrefix(ref, "id=") {
				id, err = strconv.ParseUint(ref[3:], 10, 64)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		if id == 0 {
			return nil, fmt.Errorf("no id found in message: %#v", msg)
		}
		msgs[id] = soymsg.NewMessage(id, msg.Str[0]) // TODO: Plural
	}
	return &bundle{msgs}, nil
}

func (b *bundle) Message(id uint64) *soymsg.Message {
	var msg, ok = b.messages[id]
	if !ok {
		return nil
	}
	return &msg
}
