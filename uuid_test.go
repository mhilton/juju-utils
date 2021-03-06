// Copyright 2013 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package utils_test

import (
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/utils"
)

type uuidSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&uuidSuite{})

func (*uuidSuite) TestUUID(c *gc.C) {
	uuid, err := utils.NewUUID()
	c.Assert(err, gc.IsNil)
	uuidCopy := uuid.Copy()
	uuidRaw := uuid.Raw()
	uuidStr := uuid.String()
	c.Assert(uuidRaw, gc.HasLen, 16)
	c.Assert(uuidStr, jc.Satisfies, utils.IsValidUUIDString)
	uuid[0] = 0x00
	uuidCopy[0] = 0xFF
	c.Assert(uuid, gc.Not(gc.DeepEquals), uuidCopy)
	uuidRaw[0] = 0xFF
	c.Assert(uuid, gc.Not(gc.DeepEquals), uuidRaw)
	nextUUID, err := utils.NewUUID()
	c.Assert(err, gc.IsNil)
	c.Assert(uuid, gc.Not(gc.DeepEquals), nextUUID)
}

func (*uuidSuite) TestIsValidUUIDFailsWhenNotValid(c *gc.C) {
	c.Check(utils.IsValidUUIDString("blah"), gc.Equals, false)
	c.Check(utils.IsValidUUIDString("blah-9f484882-2f18-4fd2-967d-db9663db7bea"), gc.Equals, false)
	c.Check(utils.IsValidUUIDString("9f484882-2f18-4fd2-967d-db9663db7bea-blah"), gc.Equals, false)
	c.Check(utils.IsValidUUIDString("9f484882-2f18-4fd2-967d-db9663db7bea"), gc.Equals, true)
}

func (*uuidSuite) TestUUIDFromString(c *gc.C) {
	_, err := utils.UUIDFromString("blah")
	c.Assert(err, gc.ErrorMatches, `invalid UUID: "blah"`)
	validUUID := "9f484882-2f18-4fd2-967d-db9663db7bea"
	uuid, err := utils.UUIDFromString(validUUID)
	c.Assert(err, gc.IsNil)
	c.Assert(uuid.String(), gc.Equals, validUUID)
}
