package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"os"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/yaml"

	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/deckgen"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/dataplane/parser"
	"github.com/kong/kubernetes-ingress-controller/v2/internal/store"
)

func main() {
	inDirectory := flag.String("in", ".", "input directory")
	inFile := flag.String("f", "", "input file")
	outFile := flag.String("out", "kong.yaml", "output file")
	flag.Parse()

	var files [][]byte
	if *inFile != "" {
		y, err := os.ReadFile(*inFile)
		if err != nil {
			log.Fatal("Failed reading input file", err)
		}
		files = append(files, y)
	} else {
		var err error
		files, err = readAllFilesInDirectory(*inDirectory)
		if err != nil {
			log.Fatal("Failed reading input directory", err)
		}
		log.Printf("Found %d files in %s", len(files), *inDirectory)
	}

	var yamls [][]byte
	for _, f := range files {
		f := util.ManualStrip(f)
		yamls = append(yamls, splitYAMLs(f)...)
		yamls = lo.Filter(yamls, func(yaml []byte, _ int) bool {
			return len(yaml) > 0 && len(bytes.TrimSpace(yaml)) > 0
		})
	}

	log.Printf("Found %d YAMLs", len(yamls))

	logger := logrus.New()
	cacheStores, err := store.NewCacheStoresFromObjYAMLIgnoreUnknown(yamls...)
	if err != nil {
		log.Fatal("Failed creating cache stores", err)
	}

	s := store.New(cacheStores, "kong", logger)
	p, err := parser.NewParser(logger, s)
	if err != nil {
		log.Fatal("Failed creating parser", err)
	}

	kongState, failures := p.Build()
	if len(failures) > 0 {
		log.Printf("%d failures occurred while building KongState", len(failures))
	}

	pluginsSchemasStore := pluginsSchemaStoreStub{}
	targetConfig := deckgen.ToDeckContent(context.Background(),
		logger,
		kongState,
		pluginsSchemasStore,
		[]string{},
		"3.0",
	)

	b, err := yaml.Marshal(targetConfig)
	if err != nil {
		log.Fatal("Failed marshaling to YAML", err)
	}

	err = os.WriteFile(*outFile, b, 0644)
	if err != nil {
		log.Fatal("Failed writing to file", err)
	}
}

type pluginsSchemaStoreStub struct{}

func (p pluginsSchemaStoreStub) Schema(ctx context.Context, pluginName string) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func readAllFilesInDirectory(dir string) ([][]byte, error) {
	// list all files in the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// read all files in the directory
	var allFiles [][]byte
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		b, err := os.ReadFile(dir + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		allFiles = append(allFiles, b)
	}

	return allFiles, nil
}

func splitYAMLs(yamls []byte) [][]byte {
	return bytes.Split(yamls, []byte("---"))
}
