package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"

	cli "github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func main() {
	err := run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	app := &cli.App{
		Name:  "ysm",
		Usage: "usage: todo",
		Commands: []*cli.Command{
			{
				Name:   "sort",
				Usage:  "`ysm sort file` prints to std.out",
				Action: sortCliAction,
			},
			{
				Name:  "update",
				Usage: "`cat file | ysm update` prints to std.out",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "version",
						Aliases: []string{"v"},
						Value:   "networking.k8s.io/v1",
						Usage:   "version to convert to",
					},
				},
				Action: updateIngress,
			},
			{
				Name:   "rm",
				Usage:  "`cat file | ysm rm` removes paths .metadata.creationTimestamp and .status from yaml",
				Action: rmAction,
			},
		},
	}

	return app.Run(os.Args)
}

func sortCliAction(c *cli.Context) error {
	filePath := c.Args().Get(0)
	fileReader, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer fileReader.Close()

	yamlOut, err := decodeYamlToInterfaceMap(fileReader)
	if err != nil {
		return err
	}

	out, err := encodeYamlFromInterfaceMap(yamlOut)
	if err != nil {
		return err
	}

	// update(c.Context, "", "")

	fmt.Println(out)

	return nil
}

func decodeYamlToInterfaceMap(r io.Reader) (map[string]interface{}, error) {
	var err error
	emptyReturn := map[string]interface{}{}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, r)
	if err != nil {
		return emptyReturn, err
	}

	data := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(buf.String()), &data)
	if err != nil {
		return emptyReturn, err
	}

	return data, nil
}

func encodeYamlFromInterfaceMap(data map[string]interface{}) (string, error) {
	out, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("couldn't encode data to yaml: %s", err)
	}

	return string(out), nil
}

func rmAction(c *cli.Context) error {
	return readYamlAndDelField(os.Stdin, os.Stdout, []string{"metadata.creationTimestamp", "status"})
}

func readYamlAndDelField(r io.Reader, w io.Writer, pathsOfKeysToBeDeleted []string) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}

	err = yaml.Unmarshal(b, &data)
	if err != nil {
		return err
	}

	for _, path := range pathsOfKeysToBeDeleted {
		removeByPath(data, path)
	}

	out, err := encodeYamlFromInterfaceMap(data)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(out))
	if err != nil {
		return err
	}

	return nil
}

func removeByPath(data map[string]interface{}, keyPath string) {
	pathAsSlice := strings.Split(keyPath, ".")
	currentPath, subPathSlice := pathAsSlice[0], pathAsSlice[1:]
	val := reflect.ValueOf(data)

	strings.IndexRune(keyPath, '.')

	for _, k := range val.MapKeys() {
		v := val.MapIndex(k)

		if len(pathAsSlice) == 1 && currentPath == k.String() {
			delete(data, k.String())
			continue
		}

		switch t := v.Interface().(type) {
		// If key is a JSON object (Go Map), use recursion to go deeper
		case map[string]interface{}:
			removeByPath(t, strings.Join(subPathSlice[:], "."))
		}
	}
}

func updateIngress(c *cli.Context) error {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	docs := splitManifests(string(content))

	for i, doc := range docs {
		doc, err := update(c.Context, doc, c.String("version"))
		if err != nil {
			return err
		}

		docs[i] = doc
	}

	err = output(docs)
	if err != nil {
		return err
	}

	return nil
}

func update(ctx context.Context, in, version string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl-convert", "--filename", "/dev/stdin", "--output-version", version)
	if version == "" {
		cmd = exec.CommandContext(ctx, "kubectl-convert", "--filename", "/dev/stdin")
	}

	cmd.Stdin = strings.NewReader(in)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func output(docs []string) error {
	for _, doc := range docs {
		_, err := fmt.Printf("---\n%s", doc)
		if err != nil {
			return err
		}
	}

	return nil
}

func splitManifests(yml string) []string {
	yml = strings.TrimSpace(yml)

	if strings.HasPrefix(yml, "---\n") {
		yml = "\n" + yml
	}

	if strings.HasSuffix(yml, "\n---") {
		yml += "\n"
	}

	separator := regexp.MustCompile("\n---\n")
	chunks := separator.Split(yml, -1)
	docs := []string{}

	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)

		if chunk == "" {
			continue
		}

		docs = append(docs, chunk)
	}

	return docs
}
