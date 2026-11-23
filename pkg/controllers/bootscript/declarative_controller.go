// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/clients/nodeidentity"
	"github.com/openchami/fabrica/pkg/revision"
)

// ScriptGenerator is the existing handler's boot controller contract.
type ScriptGenerator interface {
	GenerateBootScript(context.Context, string, string) (string, error)
}

// DeclarativeNodeResolver resolves current identity outside the mutation gate.
type DeclarativeNodeResolver interface {
	ResolveNodeByIdentifier(context.Context, string) (*v1.Node, error)
}

// DeclarativeRevisionReader obtains an atomic assignment and immutable revision.
type DeclarativeRevisionReader interface {
	HasAssignments(context.Context) (bool, error)
	SelectForDelivery(context.Context, v1.NodeSpec, string) (v1.BootConfigSpec, v1.BootAssignment, revision.Metadata, bool, error)
}

// DeclarativeOptions binds delivery to the same revision service used for mutations.
type DeclarativeOptions struct {
	Legacy    ScriptGenerator
	Nodes     DeclarativeNodeResolver
	Revisions DeclarativeRevisionReader
}

// DeclarativeController decorates a legacy controller without modifying its behavior.
type DeclarativeController struct {
	options DeclarativeOptions
	cache   declarativeCache
}

// NewDeclarativeController constructs the handler-compatible delivery decorator.
func NewDeclarativeController(options DeclarativeOptions) *DeclarativeController {
	return &DeclarativeController{options: options}
}

// DeliveryFailure signals an operational refusal; its cause is not safe for HTTP output.
type DeliveryFailure struct{ cause error }

func (*DeliveryFailure) Error() string   { return "declarative boot delivery unavailable" }
func (e *DeliveryFailure) Unwrap() error { return e.cause }
func (DeliveryFailure) String() string   { return "declarative boot delivery unavailable" }
func (DeliveryFailure) GoString() string { return "bootscript.DeliveryFailure{}" }

func deliveryFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &DeliveryFailure{cause: err}
}

// GenerateBootScript delegates no-match to legacy with the original profile.
// Any provider failure while assignments exist fails closed: a match cannot be excluded.
func (c *DeclarativeController) GenerateBootScript(ctx context.Context, identifier, profile string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if c.options.Legacy == nil || c.options.Revisions == nil {
		return "", deliveryFailure(errors.New("delivery not configured"))
	}
	has, err := c.options.Revisions.HasAssignments(ctx)
	if err != nil {
		return "", deliveryFailure(err)
	}
	if !has {
		return c.options.Legacy.GenerateBootScript(ctx, identifier, profile)
	}
	if c.options.Nodes == nil {
		return "", deliveryFailure(errors.New("node provider not configured"))
	}
	node, err := c.options.Nodes.ResolveNodeByIdentifier(ctx, identifier)
	if errors.Is(err, nodeidentity.ErrNotFound) && ctx.Err() == nil {
		return c.options.Legacy.GenerateBootScript(ctx, identifier, profile)
	}
	if err != nil {
		return "", deliveryFailure(err)
	}
	if node == nil {
		return "", deliveryFailure(errors.New("node provider returned no identity"))
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	selectedProfile := profile
	if requestProfile, ok := RequestProfile(ctx); ok {
		selectedProfile = requestProfile
	}
	content, assignment, rev, found, err := c.options.Revisions.SelectForDelivery(ctx, node.Spec, selectedProfile)
	if err != nil {
		return "", deliveryFailure(err)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !found {
		return c.options.Legacy.GenerateBootScript(ctx, identifier, profile)
	}
	key := declarativeCacheKey(node.Spec, assignment, content, rev, selectedProfile)
	if script, found := c.cache.get(key); found {
		return script, nil
	}
	config := &v1.BootConfiguration{}
	config.Metadata.Name = assignment.Spec.ConfigRef.Name
	config.Metadata.UID = rev.UID
	config.Spec.Kernel, config.Spec.Initrd, config.Spec.Params = content.Kernel, content.Initrd, content.Params
	script, err := (&BootScriptController{}).buildIPXEScript(config, node)
	if err != nil {
		return "", deliveryFailure(err)
	}
	ref := assignment.Spec.ConfigRef
	identity := fmt.Sprintf("# assignment=%q assignment_uid=%q reference=%q reference_uid=%q reference_revision=%q reference_selector=%q revision_uid=%q revision_name=%q digest=%q\n", assignment.Metadata.Name, assignment.Metadata.UID, ref.Name, ref.UID, ref.RevisionName, ref.Selector, rev.UID, rev.RevisionName, rev.Digest)
	script = strings.Replace(script, "#!ipxe\n", "#!ipxe\n"+identity, 1)
	c.cache.put(key, script)
	return script, nil
}
