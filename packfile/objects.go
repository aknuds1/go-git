package packfile

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strconv"
	"time"
)

type Object interface {
	Type() string
	Hash() string
}

type Hash []byte

func (h Hash) String() string {
	return hex.EncodeToString(h)
}

type Commit struct {
	Tree      Hash
	Parents   []Hash
	Author    Signature
	Committer Signature
	Message   string
	hash      string
}

func NewCommit(b []byte) (*Commit, error) {
	o := &Commit{hash: calculateHash("commit", b)}

	lines := bytes.Split(b, []byte{'\n'})
	for i := range lines {
		if len(lines[i]) > 0 {
			var err error

			split := bytes.SplitN(lines[i], []byte{' '}, 2)
			switch string(split[0]) {
			case "tree":
				o.Tree = make([]byte, 20)
				_, err = hex.Decode(o.Tree, split[1])
			case "parent":
				h := make([]byte, 20)
				_, err = hex.Decode(h, split[1])
				if err == nil {
					o.Parents = append(o.Parents, h)
				}
			case "author":
				o.Author = NewSignature(split[1])
			case "committer":
				o.Committer = NewSignature(split[1])
			}

			if err != nil {
				return nil, err
			}
		} else {
			o.Message = string(bytes.Join(append(lines[i+1:]), []byte{'\n'}))
			break
		}
	}

	return o, nil
}

func (o *Commit) Type() string {
	return "commit"
}

func (o *Commit) Hash() string {
	return o.hash
}

type Signature struct {
	Name  string
	Email string
	When  time.Time
}

func NewSignature(signature []byte) Signature {
	ret := Signature{}
	if len(signature) == 0 {
		return ret
	}

	from := 0
	state := 'n' // n: name, e: email, t: timestamp, z: timezone
	for i := 0; ; i++ {
		var c byte
		var end bool
		if i < len(signature) {
			c = signature[i]
		} else {
			end = true
		}

		switch state {
		case 'n':
			if c == '<' || end {
				if i == 0 {
					break
				}
				ret.Name = string(signature[from : i-1])
				state = 'e'
				from = i + 1
			}
		case 'e':
			if c == '>' || end {
				ret.Email = string(signature[from:i])
				i++
				state = 't'
				from = i + 1
			}
		case 't':
			if c == ' ' || end {
				t, err := strconv.ParseInt(string(signature[from:i]), 10, 64)
				if err == nil {
					ret.When = time.Unix(t, 0)
				}
				end = true
			}
		}

		if end {
			break
		}
	}

	return ret
}

func (s *Signature) String() string {
	return fmt.Sprintf("%q <%s> @ %s", s.Name, s.Email, s.When)
}

type Tree struct {
	Entries []TreeEntry
	hash    string
}

type TreeEntry struct {
	Name string
	Hash string
}

func NewTree(b []byte) (*Tree, error) {
	o := &Tree{hash: calculateHash("tree", b)}

	if len(b) == 0 {
		return o, nil
	}

	zr, e := zlib.NewReader(bytes.NewBuffer(b))
	if e == nil {
		defer zr.Close()
		var err error
		b, err = ioutil.ReadAll(zr)
		if err != nil {
			return nil, err
		}
	}

	body := b
	for {
		split := bytes.SplitN(body, []byte{0}, 2)
		split1 := bytes.SplitN(split[0], []byte{' '}, 2)

		o.Entries = append(o.Entries, TreeEntry{
			Name: string(split1[1]),
			Hash: fmt.Sprintf("%x", split[1][0:20]),
		})

		body = split[1][20:]
		if len(split[1]) == 20 {
			break
		}
	}

	return o, nil
}

func (o *Tree) Type() string {
	return "tree"
}

func (o *Tree) Hash() string {
	return o.hash
}

type Blob struct {
	Len  int
	hash string
}

func NewBlob(b []byte) (*Blob, error) {
	return &Blob{Len: len(b), hash: calculateHash("blob", b)}, nil
}

func (o *Blob) Type() string {
	return "blob"
}

func (o *Blob) Hash() string {
	return o.hash
}

func calculateHash(objType string, content []byte) string {
	header := []byte(objType)
	header = append(header, ' ')
	header = strconv.AppendInt(header, int64(len(content)), 10)
	header = append(header, 0)
	header = append(header, content...)

	return fmt.Sprintf("%x", sha1.Sum(header))
}

type ContentCallback func(hash string, content []byte)
