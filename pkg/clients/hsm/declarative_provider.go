// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/clients/nodeidentity"
	"github.com/openchami/boot-service/pkg/validation"
)

type declarativeComponent struct {
	ID      string `json:"ID"`
	Type    string `json:"Type"`
	NID     *int32 `json:"NID"`
	Role    string `json:"Role"`
	SubRole string `json:"SubRole"`
}

type declarativeMembership struct {
	ID          string    `json:"id"`
	GroupLabels []*string `json:"groupLabels"`
}

// DeclarativeNodeProvider resolves current SMD state without using the legacy cache.
type DeclarativeNodeProvider struct{ client *HSMClient }

// NewDeclarativeNodeProvider retains the configured client's transport and authentication.
func NewDeclarativeNodeProvider(client *HSMClient) *DeclarativeNodeProvider {
	return &DeclarativeNodeProvider{client: client}
}

func (p *DeclarativeNodeProvider) get(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.client.config.BaseURL, "/")+path, nil)
	if err != nil {
		return err
	}
	if err := p.client.addAuthHeader(ctx, req); err != nil {
		return err
	}
	resp, err := p.client.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SMD returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(dest)
}

// ResolveNodeByIdentifier returns an ephemeral node from fresh SMD responses.
// Missing identity is an error because assignment matching cannot be excluded.
func (p *DeclarativeNodeProvider) ResolveNodeByIdentifier(ctx context.Context, identifier string) (*v1.Node, error) {
	if p.client == nil {
		return nil, errors.New("SMD client unavailable")
	}
	var components struct {
		Components []declarativeComponent `json:"Components"`
	}
	var interfaces []HSMEthernetInterface
	var memberships []declarativeMembership
	if err := p.get(ctx, "/hsm/v2/State/Components", &components); err != nil {
		return nil, err
	}
	if err := p.get(ctx, "/hsm/v2/Inventory/EthernetInterfaces", &interfaces); err != nil {
		return nil, err
	}
	if err := p.get(ctx, "/hsm/v2/memberships?type=Node", &memberships); err != nil {
		return nil, err
	}
	if components.Components == nil || interfaces == nil || memberships == nil {
		return nil, errors.New("incomplete SMD inventory response")
	}
	requestedMAC, macErr := net.ParseMAC(identifier)
	var selected *declarativeComponent
	for i := range components.Components {
		comp := &components.Components[i]
		if comp.Type != "Node" {
			continue
		}
		match := comp.ID == identifier || (comp.NID != nil && identifier != "" && strconv.FormatInt(int64(*comp.NID), 10) == identifier)
		if macErr == nil {
			for _, iface := range interfaces {
				mac, err := net.ParseMAC(iface.MACAddress)
				if iface.ComponentID == comp.ID && err == nil && mac.String() == requestedMAC.String() {
					match = true
				}
			}
		}
		if match {
			if selected != nil {
				return nil, errors.New("ambiguous SMD node identity")
			}
			selected = comp
		}
	}
	if selected == nil {
		return nil, nodeidentity.ErrNotFound
	}
	if !validation.ValidateXName(selected.ID) || selected.NID == nil {
		return nil, errors.New("SMD node identity unavailable")
	}
	node := &v1.Node{Spec: v1.NodeSpec{XName: selected.ID, NID: *selected.NID, Role: selected.Role, SubRole: selected.SubRole}}
	for _, iface := range interfaces {
		if iface.ComponentID != selected.ID {
			continue
		}
		mac, err := net.ParseMAC(iface.MACAddress)
		if err != nil {
			return nil, errors.New("invalid SMD node interface MAC")
		}
		if node.Spec.BootMAC == "" || (macErr == nil && mac.String() == requestedMAC.String()) {
			node.Spec.BootMAC = mac.String()
		}
		node.Spec.Interfaces = append(node.Spec.Interfaces, v1.NodeInterface{MAC: mac.String(), IP: iface.IPAddress, Type: iface.Type})
	}
	seen := false
	for _, membership := range memberships {
		if membership.ID != selected.ID {
			continue
		}
		if seen {
			return nil, errors.New("duplicate SMD membership")
		}
		seen = true
		if membership.GroupLabels == nil {
			return nil, errors.New("SMD node group membership unavailable")
		}
		for _, group := range membership.GroupLabels {
			if group == nil {
				return nil, errors.New("invalid SMD node group membership")
			}
			node.Spec.Groups = append(node.Spec.Groups, *group)
		}
	}
	if !seen {
		return nil, errors.New("SMD node membership unavailable")
	}
	return node, ctx.Err()
}
