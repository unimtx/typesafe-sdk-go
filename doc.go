// Package typesafe provides an unofficial, idiomatic Go client for the TypeSafe
// HTTP API. This project is unaffiliated with TypeSafe.
//
// Build a Client with scoped options, send shared state plus independent Choice,
// Score, and Noul questions to SystemOne, and consume concrete Answer values with
// type assertions or the grouped response accessors. Network methods accept a
// context first. Clients are immutable after construction and support concurrent
// calls when caller-provided transports and loggers do too.
package typesafe
