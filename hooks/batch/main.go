/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log/slog"
	"strconv"

	_ "hook/https"

	"github.com/deckhouse/module-sdk/pkg"
	"github.com/deckhouse/module-sdk/pkg/app"
	objectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/deckhouse/module-sdk/pkg/registry"
)

const (
	SnapshotKey = "golang_versions"
)

var _ = registry.RegisterFunc(config, HandlerHook)

// Since we subscribed to ApiVersion example.io/v1, we get .spec.version (see jqFilter) as an
// object with fields 'major' and 'minor'.
type VersionInfoMetadata struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Patch int `json:"patch"`
}

const ApplyNodeJQFilter = `.spec.version`

// This hook subscribes to golang.deckhouse.io/v1 CRs and puts their versions into ConfigMap
// 'golang-versions'. The 'jqFilter' expression lets us focus only on meaningful parts of resources.
// The result of this filter will be in snapshots array named 'golang_versions'. Snapshots are in
// sync with cluster state, because by default 'kubeternetes' subscription uses all kinds of events.
// #
// Refer to Shell Operator doc for details https://flant.github.io/shell-operator/HOOKS.html
var config = &pkg.HookConfig{
	Kubernetes: []pkg.KubernetesConfig{
		{
			Name:       "golang_versions",
			APIVersion: "example.io/v1",
			Kind:       "Golang",
			JqFilter:   ApplyNodeJQFilter,
		},
	},
}

func HandlerHook(_ context.Context, input *pkg.HookInput) error {
	// From the hook run context we get the snapshots as we named it in the suscription. It will
	// always be a list if it is defined in the hook config. 'versions' here contain objects of the form
	// The slice of VersionInfoMetadata is the result of jqFilter '.spec.version', see crds/golang.yaml into version v1.
	golangVersions, err := objectpatch.UnmarshalToStruct[VersionInfoMetadata](input.Snapshots, "golang_versions")
	if err != nil {
		return err
	}

	input.Logger.Info("found golang_versions", slog.Any("golangVersions", golangVersions))

	versions := make([]string, 0, len(golangVersions))
	for _, version := range golangVersions {
		versions = append(versions, parse_snap_version(version))
	}

	// IMPORTANT: We assume that this module will be named 'echo-server' when added to Deckhouse. The
	// name of the module is used in the values reference. For now, module name in deckhouse and
	// values reference are tightly coupled.
	input.Values.Set("echoserver.internal.golangVersions", versions)

	return nil
}

func parse_snap_version(version VersionInfoMetadata) string {
	major := strconv.Itoa(version.Major)
	minor := strconv.Itoa(version.Minor)
	patch := strconv.Itoa(version.Patch)
	return major + "." + minor + "." + patch
}

func main() {
	readinessConfig := &app.ReadinessConfig{
		IntervalInSeconds: 12,
		ProbeFunc:         ReadinessFunc,
	}

	app.Run(app.WithReadiness(readinessConfig))
}
