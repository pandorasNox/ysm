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
				Name:  "rm",
				Usage: "`ysm rm file metadata.creationTimestamp,status` removes path from yaml",
				Action: func(c *cli.Context) error {
					filePath := c.Args().Get(0)
					fileReader, err := os.Open(filePath)
					if err != nil {
						log.Fatal(err)
					}
					defer fileReader.Close()

					yamlOut, err := readYamlAndDelField(fileReader, []string{"metadata.creationTimestamp", "status"})
					if err != nil {
						log.Fatal(err)
					}

					fmt.Println(yamlOut)

					return nil
				},
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

	fmt.Println(yamlOut)

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

func readYamlAndDelField(r io.Reader, pathsOfKeysToBeDeleted []string) (string, error) {
	var err error

	buf := new(strings.Builder)
	_, err = io.Copy(buf, r)
	if err != nil {
		return "", err
	}

	// dec := yaml.NewDecoder(r)
	data := map[string]interface{}{}
	// data := make([]map[string]interface{}, 0)

	err = yaml.Unmarshal([]byte(buf.String()), &data)
	if err != nil {
		return "", err
	}

	for _, path := range pathsOfKeysToBeDeleted {
		removeByPath(data, path)
	}

	fmt.Println(data)

	// for k, v := range data {
	// 	if k == "metadata" {
	// 		fmt.Println(v)
	// 	}
	// 	// fmt.Println(key)
	// 	// _, ok := sessions["moo"];
	// 	// if ok {
	// 	// 	delete(sessions, "moo");
	// 	// }
	// }

	return "", nil
}

func removeByPath(data map[string]interface{}, keyPath string) {
	pathAsSlice := strings.Split(keyPath, ".")
	currentPath, subPathSlice := pathAsSlice[0], pathAsSlice[1:]
	_ = currentPath

	val := reflect.ValueOf(data)
	// fmt.Println("MapKeys: ", val.MapKeys())
	for _, k := range val.MapKeys() {
		// fmt.Printf("k: '%s' \n", k.String())
		v := val.MapIndex(k)
		// fmt.Println("k.v: ", v)
		if len(pathAsSlice) == 1 && currentPath == k.String() {
			delete(data, k.String())
			continue
		}
		// if v.IsNil() {
		// 	delete(m, e.String())
		// 	continue
		// }
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

		// dec := yaml.NewDecoder(bytes.NewReader(d))
		// data := map[string]interface{}{}
		// if err := dec.Decode(&data); err != nil {
		// 	if err.Error() == "EOF" {
		// 		break
		// 	}
		// 	return fmt.Errorf("error reading yaml document %d: %s", i, err)
		// }

		// err = yq(doc, "del(.status)")
		// if err != nil {
		// 	return err
		// }

		// err = yq(doc, "del(.metadata.creationTimestamp)")
		// if err != nil {
		// 	return err
		// }

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

	// script := fmt.Sprintf(`cat <<< $(kubectl-convert --filename %s --output-version %s) > %s`, shellescape.Quote(filename), shellescape.Quote(outputVersion), shellescape.Quote(filename))
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

// func yq(in, action string) (string, error) {
// 	cmd := exec.Command("yq", "--yml-output", "--yml-roundtrip", "--width=160", action)
// 	cmd.Stderr = os.Stderr
// 	cmd.Stdin = strings.NewReader(in)

// 	return cmd.Output()
// }
