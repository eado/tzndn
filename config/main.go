package config

import (
	enc "github.com/named-data/ndnd/std/encoding"
)

var Endpoint = "suns.cs.ucla.edu:6363"

var MulticastPrefix, _ = enc.NameFromStr("/ndn/multicast")
var TzdbPrefix, _ = enc.NameFromStr("tz19")
var UserPrefix, _ = enc.NameFromStr("/ndn/edu/ucla/cs/omar")
var GroupPrefix = UserPrefix.Append(TzdbPrefix...)
var RepoName, _ = enc.NameFromStr("/ndnd/ucla/repo")

var Files = []string{"africa", "antarctica", "asia", "australasia", "europe", "northamerica", "southamerica", "zonenow.tab", "zone1970.tab"}

var OutputDir = "/Users/omarelamri/tzdist"
var InputDir = "/Users/omarelamri/tz"
