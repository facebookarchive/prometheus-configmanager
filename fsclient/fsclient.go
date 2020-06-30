/*
 * Copyright (c) Facebook, Inc. and its affiliates.
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package fsclient

import (
	"io/ioutil"
	"os"
)

type FSClient interface {
	WriteFile(filename string, data []byte, perm os.FileMode) error
	ReadFile(filename string) ([]byte, error)
	DeleteFile(filename string) error
	Stat(filename string) (os.FileInfo, error)

	Root() string
}

type fsclient struct {
	root string
}

func NewFSClient(root string) FSClient {
	return &fsclient{
		root: root,
	}
}

func (f *fsclient) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return ioutil.WriteFile(f.root+filename, data, perm)
}

func (f *fsclient) ReadFile(filename string) ([]byte, error) {
	return ioutil.ReadFile(f.root + filename)
}

func (f *fsclient) DeleteFile(filename string) error {
	return os.Remove(f.root + filename)
}

func (f *fsclient) Stat(filename string) (os.FileInfo, error) {
	return os.Stat(f.root + filename)
}

func (f *fsclient) Root() string {
	return f.root
}
