// Package hash implements the hash module: digests of data.
package hash

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	gohash "hash"
	"io"

	"github.com/aisk/goblin/object"
)

// The digest functions are created once so hmac can recognise the one passed
// as its algorithm by identity.
var (
	sha256Fn = digestFunction("sha256", sha256.New)
	sha512Fn = digestFunction("sha512", sha512.New)
	sha1Fn   = digestFunction("sha1", sha1.New)
	md5Fn    = digestFunction("md5", md5.New)
	hmacFn   = &object.Function{Name: "hmac", Fn: hmacDigest}
)

func Execute() (object.Object, error) {
	return object.NewModule("hash", map[string]object.Object{
		"sha256": sha256Fn,
		"sha512": sha512Fn,
		"sha1":   sha1Fn,
		"md5":    md5Fn,
		"hmac":   hmacFn,
	}, nil), nil
}

// algorithm returns the Go constructor of one of the module's digest
// functions.
func algorithm(fn *object.Function) (func() gohash.Hash, bool) {
	switch fn {
	case sha256Fn:
		return sha256.New, true
	case sha512Fn:
		return sha512.New, true
	case sha1Fn:
		return sha1.New, true
	case md5Fn:
		return md5.New, true
	}
	return nil, false
}

func digestFunction(name string, newHash func() gohash.Hash) *object.Function {
	return &object.Function{Name: name, Fn: func(args object.CallArgs) (object.Object, error) {
		p := object.NewArgParser(name, args)
		data := p.Data("data")
		if err := p.Finish(); err != nil {
			return nil, err
		}
		h := newHash()
		if err := feed(name, h, data); err != nil {
			return nil, err
		}
		return object.NewBytes(h.Sum(nil)), nil
	}}
}

// feed copies data into h in chunks, so a readable of any size is hashed
// without holding it in memory. An error the readable raised is passed on as
// it is.
func feed(name string, h gohash.Hash, data io.Reader) error {
	if _, err := io.Copy(h, data); err != nil {
		var goblinErr *object.Error
		if errors.As(err, &goblinErr) {
			return err
		}
		return object.WrapNativeError(object.IOError, name+"() cannot read data", err)
	}
	return nil
}

func hmacDigest(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("hmac", args)
	keyArg := p.Any("key")
	data := p.Data("data")
	algorithmArg := p.AnyOr("algorithm", sha256Fn)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	var key []byte
	switch k := keyArg.(type) {
	case object.String:
		key = []byte(k)
	case object.Bytes:
		key = []byte(k)
	default:
		return nil, object.NewTypeError("hmac() argument 'key' must be String or Bytes, got %s", keyArg.TypeName())
	}
	fn, _ := algorithmArg.(*object.Function)
	newHash, ok := algorithm(fn)
	if !ok {
		return nil, object.NewValueError("hmac() argument 'algorithm' must be hash.md5, hash.sha1, hash.sha256 or hash.sha512")
	}
	mac := hmac.New(newHash, key)
	if err := feed("hmac", mac, data); err != nil {
		return nil, err
	}
	return object.NewBytes(mac.Sum(nil)), nil
}
