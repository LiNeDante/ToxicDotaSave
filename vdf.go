package main

import (
	"errors"
	"strings"
)

// vdfNode is a parsed Valve KeyValues (text VDF) object. Keys are
// case-insensitive in Steam's own reader, so lookups lower-case them.
type vdfNode struct {
	values   map[string]string
	children map[string]*vdfNode
	order    []string // child keys in file order
}

func newVDFNode() *vdfNode {
	return &vdfNode{values: map[string]string{}, children: map[string]*vdfNode{}}
}

func (n *vdfNode) Get(key string) string {
	if n == nil {
		return ""
	}
	return n.values[strings.ToLower(key)]
}

func (n *vdfNode) Child(key string) *vdfNode {
	if n == nil {
		return nil
	}
	return n.children[strings.ToLower(key)]
}

// ChildKeys returns child object keys with their original casing.
func (n *vdfNode) ChildKeys() []string {
	if n == nil {
		return nil
	}
	return n.order
}

// parseVDF parses text VDF. It tolerates unquoted tokens, // comments and
// [$WIN32]-style conditionals (which are skipped).
func parseVDF(src string) (*vdfNode, error) {
	p := &vdfParser{src: src}
	root := newVDFNode()
	if err := p.parseBody(root, false); err != nil {
		return nil, err
	}
	return root, nil
}

type vdfParser struct {
	src string
	pos int
}

func (p *vdfParser) skip() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == 0xEF || c == 0xBB || c == 0xBF:
			p.pos++
		case c == '/' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '/':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		case c == '[':
			for p.pos < len(p.src) && p.src[p.pos] != ']' {
				p.pos++
			}
			p.pos++
		default:
			return
		}
	}
}

// token returns the next token; brace tokens are returned as "{" / "}" with
// isBrace set so a quoted "{" string is not mistaken for structure.
func (p *vdfParser) token() (tok string, isBrace bool, ok bool) {
	p.skip()
	if p.pos >= len(p.src) {
		return "", false, false
	}
	c := p.src[p.pos]
	if c == '{' || c == '}' {
		p.pos++
		return string(c), true, true
	}
	var b strings.Builder
	if c == '"' {
		p.pos++
		for p.pos < len(p.src) {
			c = p.src[p.pos]
			if c == '\\' && p.pos+1 < len(p.src) {
				switch p.src[p.pos+1] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(p.src[p.pos+1])
				}
				p.pos += 2
				continue
			}
			if c == '"' {
				p.pos++
				return b.String(), false, true
			}
			b.WriteByte(c)
			p.pos++
		}
		return b.String(), false, true
	}
	for p.pos < len(p.src) {
		c = p.src[p.pos]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '{' || c == '}' || c == '"' {
			break
		}
		b.WriteByte(c)
		p.pos++
	}
	return b.String(), false, true
}

func (p *vdfParser) parseBody(n *vdfNode, nested bool) error {
	for {
		key, brace, ok := p.token()
		if !ok {
			if nested {
				return errors.New("vdf: unexpected end of input")
			}
			return nil
		}
		if brace {
			if key == "}" && nested {
				return nil
			}
			return errors.New("vdf: unexpected brace")
		}
		val, brace, ok := p.token()
		if !ok {
			return errors.New("vdf: missing value for " + key)
		}
		lk := strings.ToLower(key)
		if brace && val == "{" {
			child := newVDFNode()
			if err := p.parseBody(child, true); err != nil {
				return err
			}
			if _, dup := n.children[lk]; !dup {
				n.order = append(n.order, key)
			}
			n.children[lk] = child
			continue
		}
		if brace {
			return errors.New("vdf: unexpected brace after " + key)
		}
		n.values[lk] = val
	}
}
