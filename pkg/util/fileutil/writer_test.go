/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fileutil

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sigs.k8s.io/secrets-store-csi-driver/pkg/test_utils/tmpdir"
	"sigs.k8s.io/secrets-store-csi-driver/provider/v1alpha1"
)

func TestValidate_Success(t *testing.T) {
	cases := []struct {
		name    string
		payload []*v1alpha1.File
		skipon  string
	}{
		{
			name: "valid double payload",
			payload: []*v1alpha1.File{
				{
					Path: "foo",
				},
				{
					Path: "bar",
				},
			},
		},
		{
			name: "valid single payload",
			payload: []*v1alpha1.File{
				{
					Path: "foo",
				},
			},
		},
		{
			name: "valid nested path (linux)",
			payload: []*v1alpha1.File{
				{
					Path: "foo/bar",
				},
			},
			skipon: "windows",
		},
		{
			name: "valid nested path (windows)",
			// note: on linux this will be treated as a file with name `foo\bar`
			// not a file `bar` nested in the directory `foo`.
			payload: []*v1alpha1.File{
				{
					Path: "foo\\bar",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skipon == runtime.GOOS {
				t.SkipNow()
			}
			if err := Validate(tc.payload); err != nil {
				t.Errorf("%v: unexpected error: %v", tc.name, err)
			}
		})
	}
}

func TestValidate_Error(t *testing.T) {
	maxPath := strings.Repeat("a", maxPathLength+1)

	cases := []struct {
		name    string
		payload []*v1alpha1.File
	}{
		{
			name: "payload with path length > 4096 is invalid",
			payload: []*v1alpha1.File{
				{
					Path: maxPath,
				},
			},
		},
		{
			name: "payload with absolute path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "/dev/null",
				},
			},
		},
		{
			name: "payload with reserved path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "..sneaky.txt",
				},
			},
		},
		{
			name: "payload with doubledot path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "foo/../etc/password",
				},
			},
		},
		{
			name: "payload with empty path is invalid",
			payload: []*v1alpha1.File{
				{
					Path: "",
				},
			},
		},
		{
			name: "payload with unclean path should be cleaned",
			payload: []*v1alpha1.File{
				{
					Path: "foo////bar",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if Validate(tc.payload) == nil {
				t.Error("unexpected success")
			}
		})
	}
}

func TestWritePayloads(t *testing.T) {
	cases := []struct {
		name    string
		first   []*v1alpha1.File
		second  []*v1alpha1.File
		removed []string
	}{
		{
			name: "simple payload",
			first: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "simple payload - mode",
			first: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0440,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0644,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "simple payload - mode2",
			first: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0777,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0777,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "simple payload - mode3",
			first: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0666,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo",
					Mode:     0666,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "nested path payload",
			first: []*v1alpha1.File{
				{
					Path:     "foo/bar",
					Mode:     0644,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo/bar",
					Mode:     0644,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "multiple nested paths",
			first: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("first"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("second"),
				},
			},
		},
		{
			name: "multiple nested paths and files",
			first: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("foo"),
				},
				{
					Path:     "foo/bar/2.txt",
					Mode:     0644,
					Contents: []byte("two"),
				},
				{
					Path:     "foo/1.txt",
					Mode:     0644,
					Contents: []byte("one"),
				},
				{
					Path:     "root.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "foo/bar/baz",
					Mode:     0644,
					Contents: []byte("second - foo"),
				},
				{
					Path:     "foo/bar/2.txt",
					Mode:     0644,
					Contents: []byte("second - two"),
				},
				{
					Path:     "foo/1.txt",
					Mode:     0644,
					Contents: []byte("second - one"),
				},
				{
					Path:     "root.txt",
					Mode:     0644,
					Contents: []byte("second - root"),
				},
			},
		},
		{
			name: "removed path - simple",
			first: []*v1alpha1.File{
				{
					Path:     "foo.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "bar.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			removed: []string{"foo.txt"},
		},
		{
			name: "removed path - nested",
			first: []*v1alpha1.File{
				{
					Path:     "a/foo.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "a/bar.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			removed: []string{"a/foo.txt"},
		},
		{
			name: "removed path - double nesting",
			first: []*v1alpha1.File{
				{
					Path:     "a/b/foo.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			second: []*v1alpha1.File{
				{
					Path:     "a/c/bar.txt",
					Mode:     0644,
					Contents: []byte("root"),
				},
			},
			removed: []string{"a/b/foo.txt"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tmpdir.New(t, "", "ut")

			// check that the first write succeeds and the contents match
			if err := WritePayloads(dir, tc.first); err != nil {
				t.Errorf("WritePayload(first) got error: %v", err)
			}

			if err := readPayloads(dir, tc.first); err != nil {
				t.Errorf("WritePayload(first) could not be read: %v", err)
			}

			// check that the second write succeeds and the contents match,
			// ensuring that the files have the updated values
			if err := WritePayloads(dir, tc.second); err != nil {
				t.Errorf("WritePayload(second) got error: %v", err)
			}

			if err := readPayloads(dir, tc.second); err != nil {
				t.Errorf("WritePayload(second) could not be read: %v", err)
			}

			// check that files that should be removed by the second write are
			// gone
			for i := range tc.removed {
				if _, err := os.Lstat(filepath.Join(dir, tc.removed[i])); os.IsNotExist(err) {
					continue
				}
				t.Errorf("WritePayload() did not remove file: %s", tc.removed[i])
			}
		})
	}
}

func TestWritePayloads_BackwardCompatible(t *testing.T) {
	dir := tmpdir.New(t, "", "ut")

	// write a file simulating the provider-style file writing
	path := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatalf("could not write old file: %s", err)
	}

	payload := []*v1alpha1.File{
		{
			Path:     "foo.txt",
			Mode:     0600,
			Contents: []byte("new"),
		},
	}

	want := []byte("new")

	if err := WritePayloads(dir, payload); err != nil {
		t.Fatalf("could not write new file: %s", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read payload: %s", err)
	}

	if !bytes.Equal(got, want) {
		t.Errorf("WritePayload() did not update the file contents. got: %s, want: %s", got, want)
	}
}

func readPayloads(path string, payloads []*v1alpha1.File) error {
	for _, p := range payloads {
		fp := filepath.Join(path, p.Path)
		info, err := os.Stat(fp)
		if err != nil {
			return err
		}
		if runtime.GOOS == "windows" {
			// on windows only the 0200 bitmask is used by chmod
			// https://golang.org/src/os/file.go?s=15847:15891#L522
			if (info.Mode() & 0200) != (fs.FileMode(p.Mode) & 0200) {
				return fmt.Errorf("unexpected file mode. got: %v, want: %v", info.Mode(), fs.FileMode(p.Mode))
			}
		} else {
			if info.Mode() != fs.FileMode(p.Mode) {
				return fmt.Errorf("unexpected file mode. got: %v, want: %v", info.Mode(), fs.FileMode(p.Mode))
			}
		}

		contents, err := os.ReadFile(fp)
		if err != nil {
			return err
		}
		if !bytes.Equal(contents, p.Contents) {
			return errors.New("missmatched file contents")
		}
	}
	return nil
}
