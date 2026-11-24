// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package declarativeboot

import "context"

func (m *Manager) DeleteByName(ctx context.Context, name string, c Conditions) error {
	return m.WithMutation(ctx, func(_ LockedView) error {
		if err := c.Validate(); err != nil {
			return err
		}
		id, ok := m.byName[name]
		if !ok {
			return ErrPrecondition
		}
		return m.deleteLocked(ctx, id, c)
	})
}

func (m *Manager) DeleteByUID(ctx context.Context, id string, c Conditions) error {
	return m.WithMutation(ctx, func(_ LockedView) error { return m.deleteLocked(ctx, id, c) })
}

func (m *Manager) deleteLocked(ctx context.Context, id string, c Conditions) error {
	if err := c.Validate(); err != nil {
		return err
	}
	a, exists := m.byUID[id]
	if !exists {
		return ErrPrecondition
	}
	if err := c.check(snapshot(a).ETag, true); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	delete(m.byName, a.Metadata.Name)
	delete(m.byUID, id)
	return nil
}
