// Copyright 2022 The Cluster Monitoring Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/openshift/cluster-monitoring-operator/hack/docgen/format/asciidocs"
	"github.com/openshift/cluster-monitoring-operator/hack/docgen/format/markdown"
)

func main() {
	if os.Args[1] == "markdown" {
		markdown.PrintAPIDocs(os.Args[2:])
	} else if os.Args[1] == "asciidocs" {
		asciidocs.PrintAPIDocs(os.Args[2:])
	} else {
		fmt.Println("No format for output was passed as the first argument, supported formats are: markdown or asciidocs")
	}
}
