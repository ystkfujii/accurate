package sub

import (
	"context"
	"fmt"

	accuratev2 "github.com/cybozu-go/accurate/api/accurate/v2"
	"github.com/cybozu-go/accurate/pkg/constants"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type subGraftOpts struct {
	streams genericiooptions.IOStreams
	client  client.Client
	name    string
	parent  string
}

func newSubGraftCmd(streams genericiooptions.IOStreams, config *genericclioptions.ConfigFlags) *cobra.Command {
	opts := &subGraftOpts{}

	cmd := &cobra.Command{
		Use:   "graft NS PARENT",
		Short: "Convert a non-sub-namespace NS to a sub-namespace of PARENT",
		Long: `Convert a non-sub-namespace NS to a sub-namespace of PARENT.
NS must not be a sub-namespace.
PARENT must be either a root or a sub-namespace.

If NS is set to "root" or "template", the type will be cleared.
Also, if a template is set, it will be cleared.

A SubNamespace resource will be created in the PARENT namespace.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Fill(streams, config, args); err != nil {
				return err
			}
			return opts.Run(cmd.Context())
		},
	}

	return cmd
}

func (o *subGraftOpts) Fill(streams genericiooptions.IOStreams, config *genericclioptions.ConfigFlags, args []string) error {
	o.streams = streams
	cl, err := makeClient(config)
	if err != nil {
		return err
	}
	o.client = cl
	o.name = args[0]
	o.parent = args[1]
	return nil
}

func (o *subGraftOpts) Run(ctx context.Context) error {
	ns := &corev1.Namespace{}
	if err := o.client.Get(ctx, client.ObjectKey{Name: o.name}, ns); err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", o.name, err)
	}

	if _, ok := ns.Labels[constants.LabelParent]; ok {
		return fmt.Errorf("%s is a sub-namespace", o.name)
	}

	// Use Server-Side Apply to update the namespace labels
	ac := corev1ac.Namespace(o.name).
		WithLabels(map[string]string{
			constants.LabelParent: o.parent,
		})
	if err := o.client.Apply(ctx, ac, fieldOwner, client.ForceOwnership); err != nil {
		return fmt.Errorf("failed to apply namespace %s: %w", o.name, err)
	}

	sn := &accuratev2.SubNamespace{}
	sn.Namespace = o.parent
	sn.Name = o.name
	if err := o.client.Create(ctx, sn); err != nil {
		return fmt.Errorf("failed to create SubNamespace %s/%s", o.parent, o.name)
	}

	fmt.Fprintf(o.streams.Out, "grafted %s under %s\n", o.name, o.parent)
	return nil
}
