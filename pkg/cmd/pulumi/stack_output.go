// Copyright 2016-2018, Pulumi Corporation.
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
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newStackOutputCmd() *cobra.Command {
	var socmd stackOutputCmd
	cmd := &cobra.Command{
		Use:   "output [property-name]",
		Args:  cmdutil.MaximumNArgs(1),
		Short: "Show a stack's output properties",
		Long: "Show a stack's output properties.\n" +
			"\n" +
			"By default, this command lists all output properties exported from a stack.\n" +
			"If a specific property-name is supplied, just that property's value is shown.",
		Run: cmdutil.RunFunc(func(cmd *cobra.Command, args []string) error {
			return socmd.Run(commandContext(), args)
		}),
	}

	cmd.PersistentFlags().BoolVarP(
		&socmd.jsonOut, "json", "j", false, "Emit output as JSON")
	cmd.PersistentFlags().StringVarP(
		&socmd.stackName, "stack", "s", "", "The name of the stack to operate on. Defaults to the current stack")
	cmd.PersistentFlags().BoolVar(
		&socmd.showSecrets, "show-secrets", false, "Display outputs which are marked as secret in plaintext")

	return cmd
}

type stackOutputCmd struct {
	stackName   string
	showSecrets bool
	jsonOut     bool
}

func (cmd *stackOutputCmd) Run(ctx context.Context, args []string) error {
	opts := display.Options{
		Color: cmdutil.GetGlobalColorization(),
	}

	// Fetch the current stack and its output properties.
	s, err := requireStack(ctx, cmd.stackName, stackLoadOnly, opts)
	if err != nil {
		return err
	}
	snap, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}

	outputs, err := getStackOutputs(snap, cmd.showSecrets)
	if err != nil {
		return fmt.Errorf("getting outputs: %w", err)
	}
	if outputs == nil {
		outputs = make(map[string]interface{})
	}

	// If there is an argument, just print that property.  Else, print them all (similar to `pulumi stack`).
	if len(args) > 0 {
		name := args[0]
		v, has := outputs[name]
		if has {
			if cmd.jsonOut {
				if err := printJSON(v); err != nil {
					return err
				}
			} else {
				fmt.Printf("%v\n", stringifyOutput(v))
			}
		} else {
			return fmt.Errorf("current stack does not have output property '%v'", name)
		}
	} else if cmd.jsonOut {
		if err := printJSON(outputs); err != nil {
			return err
		}
	} else {
		printStackOutputs(outputs)
	}

	if cmd.showSecrets {
		log3rdPartySecretsProviderDecryptionEvent(ctx, s, "", "pulumi stack output")
	}

	return nil
}

func getStackOutputs(snap *deploy.Snapshot, showSecrets bool) (map[string]interface{}, error) {
	state, err := stack.GetRootStackResource(snap)
	if err != nil {
		return nil, err
	}

	if state == nil {
		return map[string]interface{}{}, nil
	}

	// massageSecrets will remove all the secrets from the property map, so it should be safe to pass a panic
	// crypter. This also ensure that if for some reason we didn't remove everything, we don't accidentally disclose
	// secret values!
	return stack.SerializeProperties(display.MassageSecrets(state.Outputs, showSecrets),
		config.NewPanicCrypter(), showSecrets)
}
