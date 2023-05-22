// Copyright 2023 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package callbacks

import (
	"fmt"
	"io"
	"os"
	"time"
)

const (
	logFilePermissions = 0666
)

type Callback interface {
	Call(string, string, string) error // takes data
	CleanUp() error
}

type StdoutCallBack struct {
}

func (c StdoutCallBack) Call(collectorName, datatype, line string) error {
	fmt.Printf("%v, %v:%v, %v\n", time.Now().UTC(), collectorName, datatype, line) //nolint:forbidigo // the point of this callback is to print
	return nil
}

func (c StdoutCallBack) CleanUp() error {
	return nil
}

func NewFileCallback(filename string) (FileCallBack, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, logFilePermissions)
	if err != nil {
		return FileCallBack{}, fmt.Errorf("failed to open file: %w", err)
	}
	return FileCallBack{FileHandle: file}, nil
}

type FileCallBack struct {
	FileHandle io.WriteCloser
}

func (c FileCallBack) Call(collectorName, datatype, line string) error {
	output := fmt.Sprintf("%v, %v:%v, %v\n", time.Now().UTC(), collectorName, datatype, line)
	_, err := c.FileHandle.Write([]byte(output))
	if err != nil {
		return fmt.Errorf("failed to write to file in callback: %w", err)
	}
	return nil
}

func (c FileCallBack) CleanUp() error {
	err := c.FileHandle.Close()
	if err != nil {
		return fmt.Errorf("failed to close file handle in callback: %w", err)
	}
	return nil
}
