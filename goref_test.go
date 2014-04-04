package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAll(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Error("fail to get current working directory.")
		return
	}
	wd, err = filepath.EvalSymlinks(wd)
	if err != nil {
		t.Error("fail to resolve symlinks for current working directory.")
		return
	}

	testsRoot := filepath.Join(wd, "tests")
	err = filepath.Walk(testsRoot, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if !strings.HasPrefix(base, "ts_") || !strings.HasSuffix(base, ".json") {
			return nil
		}

		// ignore un-finished test case
		if strings.Contains(base, "todo") {
			fmt.Println("Ignore\t", base)
			return nil
		}

		fmt.Println("Run\t", base)
		return runTest(path, wd, t)
	})

	if err != nil {
		t.Error("fail,", err)
		t.Fail()
	}
}

type Configuration struct {
	File   string
	Offset int
	Path   string

	Expected map[string]bool
}

type ConfigJson map[string]interface{}

func NewConfiguration(json *ConfigJson) *Configuration {
	config := &Configuration{Expected: make(map[string]bool)}

	for k, v := range *json {
		kk := strings.ToLower(k)
		switch {
		case kk == "file":
			config.File = v.(string)
		case kk == "offset":
			config.Offset = int(v.(float64))
		case kk == "path":
			config.Path = v.(string)
		case kk == "expected":
			for _, vv := range v.([]interface{}) {
				exp := vv.(string)
				config.Expected[exp] = true
			}
		}
	}

	return config
}

func (c *Configuration) String() string {
	s := fmt.Sprintf("file: %s, ", c.File)
	s += fmt.Sprintf("offset: %d, ", c.Offset)
	s += fmt.Sprintf("path: %s, ", c.Path)
	s += fmt.Sprintf("expected: %v, ", c.Expected)
	return s
}

func (c *Configuration) Prepare(pathPrefix string) (err error) {
	c.File = filepath.Join(pathPrefix, c.File)
	c.Path = filepath.Join(pathPrefix, c.Path)

	// You cannot manipulate keys while traverse them,
	// make a deep copy to get key list, then manipulate its map
	source := c.Exps()
	var exps []string = make([]string, len(source))
	if n := copy(exps, source); n != len(source) {
		return errors.New("builtin copy() failed")
	}

	for _, k := range exps {
		delete(c.Expected, k)

		kk := filepath.Join(pathPrefix, k)
		c.Expected[kk] = true
	}

	return
}

func (c *Configuration) Pass(output string) bool {
	results := strings.Split(strings.TrimSpace(output), "\n")
	if len(results) != len(c.Expected) {
		return false
	}

	for _, result := range results {
		if _, ok := c.Expected[result]; !ok {
			return false
		}
	}

	return true
}

func (c *Configuration) Exps() []string {
	var expected []string

	for k := range c.Expected {
		expected = append(expected, k)
	}

	return expected
}

func runTest(test string, wd string, t *testing.T) error {
	base := filepath.Base(test)

	data, err := ioutil.ReadFile(test)
	if err != nil {
		failAt(t, base, "read config file", err)
		return nil
	}

	var f interface{}
	if err = json.Unmarshal(data, &f); err != nil {
		failAt(t, base, "unmarshal json", err)
		return nil
	}

	gorefPath := "goref"
	for _, configJson := range f.([]interface{}) {
		configJson := ConfigJson(configJson.(map[string]interface{}))
		config := NewConfiguration(&configJson)
		if err = config.Prepare("tests"); err != nil {
			failAt(t, base, "unify names", err)
			return nil
		}

		output, errout, err := runGorefCmd(gorefPath, config)
		if err != nil {
			failAt(t, base, "run 'goref' command", err)
			t.Fatal("something seriously wrong")
			return err
		}

		if errout != "" || !config.Pass(output) {
			if errout != "" {
				output = errout
			}

			msg := fmt.Sprintf("\texpected: \n\t\t%s\n", strings.Join(config.Exps(), "\n\t\t"))
			msg += fmt.Sprintf("\tactual: \n\t\t%s\n", indentOutput(output))
			name := fmt.Sprintf("%s, testcase #%v, name: '%v'", base, configJson["seq"], configJson["name"])
			reportFailedTest(t, name, msg)
		}
	}

	t.Log("\n", base, "PASSED")
	return nil
}

func runGorefCmd(gorefPath string, config *Configuration) (string, string, error) {
	command := exec.Command(gorefPath, "-R", "-f", config.File, "-o", fmt.Sprintf("%d", config.Offset), config.Path)
	stdout, err := command.StdoutPipe()
	if err != nil {
		msg := fmt.Sprintf("failed to get stdout of 'goref' command, %v", err)
		return "", "", errors.New(msg)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		msg := fmt.Sprintf("failed to get stderr of 'goref' command, %v", err)
		return "", "", errors.New(msg)
	}
	if err = command.Start(); err != nil {
		msg := fmt.Sprintf("failed to start 'goref' command, %v", err)
		return "", "", errors.New(msg)
	}

	output, err := readPipe(stdout)
	if err != nil {
		msg := fmt.Sprintf("failed to read stdout of 'goref' command, %v", err)
		return "", "", errors.New(msg)
	}
	errout, err := readPipe(stderr)
	if err != nil {
		msg := fmt.Sprintf("failed to read stderr of 'goref' command, %v", err)
		return "", "", errors.New(msg)
	}

	if err = command.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			msg := fmt.Sprintf("'goref' command exit with error, %v", err)
			return "", "", errors.New(msg)
		}
	}

	return output, errout, nil
}

func readPipe(reader io.ReadCloser) (output string, err error) {
	obytes := make([]byte, 256)
	for done := false; !done; {
		var n int
		n, err = reader.Read(obytes)
		if n > 0 {
			output += string(obytes[:n])
		}

		done = err != nil
	}

	if err != io.EOF {
		return "", err
	}

	return output, nil
}

func failAt(t *testing.T, name string, step string, err error) {
	msg := fmt.Sprintf("\tat %s, %v", step, err)
	reportFailedTest(t, name, msg)
	t.Fail()
}

func indentOutput(str string) string {
	return strings.Replace(str, "\n", "\n\t\t", -1)
}

func reportFailedTest(t *testing.T, name string, msg string) {
	desc := fmt.Sprintf("\n%s FAILED\n", name)
	desc += fmt.Sprintf("%s\n", msg)
	t.Errorf(desc)
}
