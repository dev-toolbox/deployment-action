package syncer

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type File struct {
	Path    string
	Content []byte
	SHA     string
}

func SourceExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func LoadLocalFiles(root string, placeholder string, version string) ([]File, error) {
	files := make([]File, 0)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		files = append(files, File{
			Path:    filepath.ToSlash(rel),
			Content: replacePlaceholder(content, placeholder, version),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i int, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

func PrefixFiles(prefix string, files []File) []File {
	prefixed := make([]File, 0, len(files))
	for _, file := range files {
		prefixed = append(prefixed, File{
			Path:    strings.TrimPrefix(filepath.ToSlash(filepath.Join(prefix, file.Path)), "/"),
			Content: file.Content,
			SHA:     file.SHA,
		})
	}

	return prefixed
}

func DiffExisting(desired []File, existing []File) (toUpsert []File, toDelete []File) {
	desiredMap := make(map[string]File, len(desired))
	for _, file := range desired {
		desiredMap[file.Path] = file
	}

	existingMap := make(map[string]File, len(existing))
	for _, file := range existing {
		existingMap[file.Path] = file
	}

	for _, file := range desired {
		existingFile, ok := existingMap[file.Path]
		if !ok || !bytes.Equal(existingFile.Content, file.Content) {
			if ok {
				file.SHA = existingFile.SHA
			}
			toUpsert = append(toUpsert, file)
		}
	}

	for _, file := range existing {
		if _, ok := desiredMap[file.Path]; !ok {
			toDelete = append(toDelete, file)
		}
	}

	sort.Slice(toUpsert, func(i int, j int) bool {
		return toUpsert[i].Path < toUpsert[j].Path
	})
	sort.Slice(toDelete, func(i int, j int) bool {
		return toDelete[i].Path > toDelete[j].Path
	})

	return toUpsert, toDelete
}

func replacePlaceholder(content []byte, placeholder string, version string) []byte {
	if placeholder == "" || !utf8.Valid(content) {
		return content
	}

	return bytes.ReplaceAll(content, []byte(placeholder), []byte(version))
}
