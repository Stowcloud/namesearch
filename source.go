package namesearch

import "strings"

type Namespace uint32

type EntryKind uint8

const (
	EntryOther EntryKind = iota
	EntryFile
	EntryDir
	EntrySymlink
)

func (k EntryKind) IsDir() bool { return k == EntryDir }

type Entry struct {
	Name string
	Kind EntryKind
	Ino  uint64
}

type Stat struct {
	Dev, Ino, Size uint64
	MTimeNs        int64
	Kind           EntryKind
}

type Reader interface {
	ReadDir(path string, visit func(Entry) bool) error
	Stat(path string) (Stat, error)
}

// Source is the storage-neutral input to search. Adapters own conversion from
// their storage handles into Reader and plain paths before calling search.
type Source struct {
	Namespace Namespace
	Reader    Reader
	Base      string
	IndexBase string
	Prefix    string
	Allow     func(string, bool) bool
}

func NamespaceOf(id uint32) Namespace { return Namespace(id) }

func (s Source) namespace() Namespace { return s.Namespace }
func (s Source) baseString() string   { return s.Base }
func (s Source) indexBaseString() string {
	if s.IndexBase != "" {
		return s.IndexBase
	}
	return s.Base
}
func (s Source) reader() Reader { return s.Reader }
func (s Source) allow(path string, isDir bool) bool {
	return s.Allow == nil || s.Allow(path, isDir)
}
func (s Source) NamespaceOf() Namespace              { return s.namespace() }
func (s Source) BasePath() string                    { return s.baseString() }
func (s Source) IndexBasePath() string               { return s.indexBaseString() }
func (s Source) ReaderOf() Reader                    { return s.reader() }
func (s Source) Allows(path string, isDir bool) bool { return s.allow(path, isDir) }
func JoinDisplay(prefix, path string) string {
	if prefix == "" {
		return path
	}
	if path == "" {
		return strings.TrimSuffix(prefix, "/")
	}
	return prefix + path
}
