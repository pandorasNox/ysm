package main

import (
	"context"
	"fmt"
	"io"
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
				Name:  "update",
				Usage: "`cat file | ysm update` prints to std.out",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "version",
						Aliases: []string{"v"},
						Value:   "networking.k8s.io/v1",
						Usage:   "version to convert to",
					},
					&cli.StringFlag{
						Name:    "cleanup",
						Aliases: []string{"c"},
						Value:   "metadata.creationTimestamp,status",
						Usage:   "remove paths .metadata.creationTimestamp and .status by default",
					},
				},
				Action: updateIngress,
			},
		},
	}

	return app.Run(os.Args)
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

func readYamlAndDelField(in []byte, pathsOfKeysToBeDeleted []string) (string, error) {
	data := map[string]interface{}{}

	err := yaml.Unmarshal(in, &data)
	if err != nil {
		return "", err
	}

	for _, path := range pathsOfKeysToBeDeleted {
		removeByPath(data, path)
	}

	out, err := encodeYamlFromInterfaceMap(data)
	if err != nil {
		return "", err
	}

	return out, nil
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

func encodeYamlFromInterfaceMap(data map[string]interface{}) (string, error) {
	b := strings.Builder{}
	yamlEncoder := yaml.NewEncoder(&b)

	yamlEncoder.SetIndent(2)

	err := yamlEncoder.Encode(&data)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

func updateIngress(c *cli.Context) error {
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	rmPaths := strings.Split(c.String("cleanup"), ",")

	if c.String("cleanup") == "" {
		rmPaths = []string{}
	}

	docs := splitManifests(string(content))

	for i, doc := range docs {
		doc, err := update(c.Context, doc, c.String("version"))
		if err != nil {
			return err
		}

		docs[i] = doc
	}

	err = output(docs, rmPaths)
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

func output(docs, rmPaths []string) error {
	for _, doc := range docs {
		doc, err := readYamlAndDelField([]byte(doc), rmPaths)
		if err != nil {
			return err
		}

		_, err = fmt.Printf("---\n%s", doc)
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
