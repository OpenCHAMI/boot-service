// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"context"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
)

// DeclarativeLocalNodeProvider resolves nodes from the existing local source.
type DeclarativeLocalNodeProvider struct{ controller BootScriptController }

// NewDeclarativeLocalNodeProvider wraps the configured local boot client.
func NewDeclarativeLocalNodeProvider(client client.Client) *DeclarativeLocalNodeProvider {
	return &DeclarativeLocalNodeProvider{controller: BootScriptController{client: client}}
}

// ResolveNodeByIdentifier uses the existing local identifier resolution semantics.
func (p *DeclarativeLocalNodeProvider) ResolveNodeByIdentifier(ctx context.Context, identifier string) (*v1.Node, error) {
	return p.controller.resolveNode(ctx, p.controller.parseNodeIdentifier(identifier))
}
