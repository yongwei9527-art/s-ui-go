package core

import "errors"

var errDynamicCoreUpdateUnsupported = errors.New("sing-box v1.14.3 requires a full core restart for component updates")

func (c *Core) AddInbound(_ []byte) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) RemoveInbound(_ string) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) AddOutbound(_ []byte) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) RemoveOutbound(_ string) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) AddEndpoint(_ []byte) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) RemoveEndpoint(_ string) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) AddService(_ []byte) error {
	return errDynamicCoreUpdateUnsupported
}

func (c *Core) RemoveService(_ string) error {
	return errDynamicCoreUpdateUnsupported
}
