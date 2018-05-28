// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package generate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-openapi/spec"
	"github.com/go-swagger/go-swagger/scan"
	flags "github.com/jessevdk/go-flags"
)

var XJdcloudModule string

// SpecFileJD command to generate a swagger spec from a go application
type SpecFileJD struct {
	BasePath   string `long:"base-path" short:"b" description:"the base path to use" default:"."`
	BuildTags  string `long:"tags" short:"t" description:"build tags" default:""`
	ScanModels bool   `long:"scan-models" short:"m" description:"includes models that were annotated with 'swagger:model'"`
	Compact    bool   `long:"compact" description:"when present, doesn't prettify the json"`
	// Output         flags.Filename `long:"output" short:"o" description:"the file to write to"`
	Input          flags.Filename `long:"input" short:"i" description:"the file to use as input"`
	XJdcloudModule string         `long:"XJdcloudModule" short:"j" description:"XJdcloudModule"`
}

// Execute runs this command
func (s *SpecFileJD) Execute(args []string) error {
	input, err := loadSpec(string(s.Input))
	if err != nil {
		return err
	}

	XJdcloudModule = s.XJdcloudModule

	var opts scan.Opts
	opts.BasePath = s.BasePath
	opts.Input = input
	opts.ScanModels = s.ScanModels
	opts.BuildTags = s.BuildTags
	swspec, err := scan.Application(opts)
	if err != nil {
		return err
	}

	formatForJD(swspec)

	return writeToFileForJD(swspec, !s.Compact)
}

func writeToFileForJD(swspec *spec.Swagger, pretty bool) error {

	rootPath := "./"
	servicePath := fmt.Sprintf("%s/service", rootPath)
	serviceFile := fmt.Sprintf("%s/swagger.json", servicePath)
	modelsPath := fmt.Sprintf("%s/model", rootPath)
	modelsFile := fmt.Sprintf("%s/models.json", modelsPath)

	os.MkdirAll(servicePath, 0777)
	os.MkdirAll(modelsPath, 0777)

	var serviceb []byte
	var modelsb []byte
	var err error

	serviceSpec := *swspec
	modelsSpec := new(spec.Swagger)

	serviceSpec.Definitions = nil

	modelsSpec.Definitions = swspec.Definitions
	modelsSpec.Swagger = swspec.Swagger

	if pretty {
		serviceb, err = json.MarshalIndent(serviceSpec, "", "  ")
		modelsb, err = json.MarshalIndent(modelsSpec, "", "  ")
	} else {
		serviceb, err = json.Marshal(serviceSpec)
		modelsb, err = json.Marshal(modelsSpec)
	}
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(serviceFile, serviceb, 0644)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(modelsFile, modelsb, 0644)

}

func formatForJD(swspec *spec.Swagger) {

	if len(XJdcloudModule) > 0 {
		for k := range swspec.Definitions {
			item := swspec.Definitions[k]
			item.XJdcloudModule = XJdcloudModule
			swspec.Definitions[k] = item
		}
	}

	for k, v := range swspec.Paths.Paths {
		swspec.Paths.Paths[k] = setXJD(swspec, v)
	}

	swspec.Responses = nil

}

func setXJD(swspec *spec.Swagger, item spec.PathItem) spec.PathItem {

	doRemovego := func(extensions spec.Extensions) spec.Extensions {
		for k := range extensions {
			if k == "x-go-name" {
				delete(extensions, k)
			}
		}
		return extensions
	}

	doReplace := func(itemOperation *spec.Operation) {
		for k, v := range itemOperation.Responses.StatusCodeResponses {

			if k == 200 {
				strs := strings.Split(v.Ref.Ref.GetURL().String(), "/")
				objName := strs[len(strs)-1]

				if _, ok := swspec.Responses[objName]; ok {
					itemOperation.Responses.StatusCodeResponses[k] = swspec.Responses[objName]
				} else {
					fmt.Printf("【ERROR】get obj %s error\n", objName)
				}
			} else {
				delete(itemOperation.Responses.StatusCodeResponses, k)
			}

		}

		f := false
		for k, v := range itemOperation.Parameters {
			if v.In == "body" {
				itemOperation.Parameters[k].XJdcloudTiered = &f
			}

			itemOperation.Parameters[k].Extensions = doRemovego(v.Extensions)
		}

	}

	for k, v := range swspec.Definitions {
		for k1 := range v.Extensions {
			if k1 == "x-go-package" {
				delete(v.Extensions, k1)
			}
		}

		for k2, v2 := range v.Properties {
			v2.Extensions = doRemovego(v2.Extensions)
			swspec.Definitions[k].Properties[k2] = v2
		}

		swspec.Definitions[k] = v
	}

	for k, v := range swspec.Responses {
		if v.Schema != nil {
			for k1, v1 := range v.Schema.Properties {
				v1.Extensions = doRemovego(v1.Extensions)
				v.Schema.Properties[k1] = v1
			}
		}
		swspec.Responses[k] = v
	}

	if item.Post != nil {
		doReplace(item.Post)
	}
	if item.Patch != nil {
		doReplace(item.Patch)
	}
	if item.Get != nil {
		doReplace(item.Get)
	}
	if item.Delete != nil {
		doReplace(item.Delete)
	}

	return item

}
