package geocode

import "strings"

// CountyCentroid returns the approximate geographic centroid for a US county.
// county and state are case-insensitive. Returns false when the county is not in the dataset.
func CountyCentroid(county, state string) (Result, bool) {
	r, ok := countyCentroids[countyKey(county, state)]
	return r, ok
}

func countyKey(county, state string) string {
	return strings.ToLower(strings.TrimSpace(county)) + "," + strings.ToLower(strings.TrimSpace(state))
}

// countyCentroids maps "county,state" to an approximate centroid Result.
// Idaho counties are fully covered. Other states are minimal.
var countyCentroids = map[string]Result{
	// Idaho — all 44 counties
	"ada,id":        {Lat: 43.4026, Lng: -116.2023, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"adams,id":      {Lat: 44.8887, Lng: -116.4451, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"bannock,id":    {Lat: 42.6617, Lng: -112.1997, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"bear lake,id":  {Lat: 42.2829, Lng: -111.3432, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"benewah,id":    {Lat: 47.2387, Lng: -116.6091, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"bingham,id":    {Lat: 43.1776, Lng: -112.4166, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"blaine,id":     {Lat: 43.4167, Lng: -114.1667, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"boise,id":      {Lat: 43.9273, Lng: -115.7291, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"bonner,id":     {Lat: 48.2726, Lng: -116.5229, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"bonneville,id": {Lat: 43.5278, Lng: -111.9296, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"boundary,id":   {Lat: 48.7228, Lng: -116.4624, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"butte,id":      {Lat: 43.7985, Lng: -113.1666, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"camas,id":      {Lat: 43.4611, Lng: -114.8282, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"canyon,id":     {Lat: 43.6220, Lng: -116.6875, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"caribou,id":    {Lat: 42.7649, Lng: -111.5954, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"cassia,id":     {Lat: 42.2618, Lng: -113.5941, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"clark,id":      {Lat: 44.2498, Lng: -112.3584, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"clearwater,id": {Lat: 46.5817, Lng: -115.6126, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"custer,id":     {Lat: 44.2418, Lng: -114.5014, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"elmore,id":     {Lat: 43.1742, Lng: -115.4719, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"franklin,id":   {Lat: 42.1004, Lng: -111.7817, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"fremont,id":    {Lat: 44.2284, Lng: -111.5449, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"gem,id":        {Lat: 44.0613, Lng: -116.3802, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"gooding,id":    {Lat: 42.9394, Lng: -114.7284, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"idaho,id":      {Lat: 45.2970, Lng: -115.4695, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"jefferson,id":  {Lat: 43.8884, Lng: -112.4137, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"jerome,id":     {Lat: 42.6680, Lng: -114.5196, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"kootenai,id":   {Lat: 47.6574, Lng: -116.6741, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"latah,id":      {Lat: 46.7740, Lng: -116.9000, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"lemhi,id":      {Lat: 44.8938, Lng: -113.9234, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"lewis,id":      {Lat: 46.2405, Lng: -116.4680, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"lincoln,id":    {Lat: 42.9840, Lng: -114.1505, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"madison,id":    {Lat: 43.7978, Lng: -111.6497, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"minidoka,id":   {Lat: 42.8763, Lng: -113.6414, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"nez perce,id":  {Lat: 46.4079, Lng: -116.7876, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"oneida,id":     {Lat: 42.1878, Lng: -112.5174, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"owyhee,id":     {Lat: 42.5869, Lng: -116.2004, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"payette,id":    {Lat: 44.0568, Lng: -116.9300, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"power,id":      {Lat: 42.6639, Lng: -112.8217, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"shoshone,id":   {Lat: 47.2618, Lng: -115.9208, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"teton,id":      {Lat: 43.7478, Lng: -111.2000, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"twin falls,id": {Lat: 42.4097, Lng: -114.6700, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"valley,id":     {Lat: 44.7197, Lng: -115.5442, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
	"washington,id": {Lat: 44.4674, Lng: -116.7851, Precision: PrecisionCountyCentroid, Provider: "builtin", Confidence: 0.5},
}
