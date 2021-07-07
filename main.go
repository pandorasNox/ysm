package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	cli "github.com/urfave/cli/v2"
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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Value:   "networking.k8s.io/v1",
				Usage:   "version to convert to",
			},
		},
		Action: updateIngress,
		// Commands: []*cli.Command{
		// 	{
		// 		Name:   "split",
		// 		Usage:  "`ysm split file`",
		// 		Action: splitCliAction,
		// 	},
		// 	{
		// 		Name:  "merge",
		// 		Usage: "`ysm merge file1 file2` or `ysm merge file-*`",
		// 	},
		// },
	}

	return app.Run(os.Args)
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

		err = yq(doc, "del(.status)")
		if err != nil {
			return err
		}

		err = yq(doc, "del(.metadata.creationTimestamp)")
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

// func splitCliAction(c *cli.Context) error {
// 	filePath := c.Args().Get(0)
// 	d, err := ioutil.ReadFile(filePath)
// 	if err != nil {
// 		return err
// 	}

// 	log.Printf("splitting %s...", filePath)

// 	dec := yaml.NewDecoder(bytes.NewReader(d))

// 	names := map[string]int{}
// 	i := 0

// 	for {
// 		data := map[string]interface{}{}
// 		if err := dec.Decode(&data); err != nil {
// 			if err.Error() == "EOF" {
// 				break
// 			}
// 			return fmt.Errorf("error reading yaml document %d: %s", i, err)
// 		}

// 		// skip empty documents
// 		if len(data) < 1 {
// 			continue
// 		}

// 		p, err := yaml.Marshal(data)
// 		if err != nil {
// 			return fmt.Errorf("error creating yaml for document %d: %s", i, err)
// 		}

// 		// deduce the name of the
// 		kind, ok := data["kind"].(string)
// 		if !ok {
// 			return fmt.Errorf("no `Kind` field specified for yaml document %d in this file.", i)
// 		}

// 		metadata, ok := data["metadata"].(map[string]interface{})
// 		if !ok {
// 			return fmt.Errorf("no `Metadata` field specified for yaml document %d in this file.", i)
// 		}

// 		n, ok := metadata["name"].(string)
// 		if !ok {
// 			return fmt.Errorf("no `Metadata.name` field specified for yaml document %d in this file.", i)
// 		}

// 		name := fmt.Sprintf("%s-%s", kind, n)

// 		c, _ := names[name]
// 		names[name] = c + 1

// 		fName := fmt.Sprintf("%s_%d.yaml", strings.ToLower(name), c)
// 		if c == 0 {
// 			fName = fmt.Sprintf("%s.yaml", strings.ToLower(name))
// 		}

// 		log.Println("Writing file:", fName)

// 		err = ioutil.WriteFile(filepath.Join(".", fName), p, 0644)
// 		if err != nil {
// 			log.Fatal("error writing file: ", err)
// 		}
// 		i++
// 	}

// 	return nil
// }

// brew install python-yq

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

func yq(in, action string) (string, error) {
	cmd := exec.Command("yq", "--yml-output", "--yml-roundtrip", "--width=160", action)
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(in)

	return cmd.Output()
}
