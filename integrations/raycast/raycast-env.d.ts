/// <reference types="@raycast/api">

/* 🚧 🚧 🚧
 * This file is auto-generated from the extension's manifest.
 * Do not modify manually. Instead, update the `package.json` file.
 * 🚧 🚧 🚧 */

/* eslint-disable @typescript-eslint/ban-types */

type ExtensionPreferences = {}

/** Preferences accessible in all the extension's commands */
declare type Preferences = ExtensionPreferences

declare namespace Preferences {
  /** Preferences accessible in the `disk-space` command */
  export type DiskSpace = ExtensionPreferences & {}
  /** Preferences accessible in the `clean-up` command */
  export type CleanUp = ExtensionPreferences & {}
  /** Preferences accessible in the `health-check` command */
  export type HealthCheck = ExtensionPreferences & {}
  /** Preferences accessible in the `what-grew` command */
  export type WhatGrew = ExtensionPreferences & {}
}

declare namespace Arguments {
  /** Arguments passed to the `disk-space` command */
  export type DiskSpace = {}
  /** Arguments passed to the `clean-up` command */
  export type CleanUp = {}
  /** Arguments passed to the `health-check` command */
  export type HealthCheck = {}
  /** Arguments passed to the `what-grew` command */
  export type WhatGrew = {}
}

