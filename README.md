# shipkit

> Toolkit de Go para construir apps de terminal que se ven bien y se instalan, actualizan y diagnostican solas.

[![Go Reference](https://pkg.go.dev/badge/github.com/fede-iglesias/shipkit.svg)](https://pkg.go.dev/github.com/fede-iglesias/shipkit)
[![CI](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml/badge.svg)](https://github.com/fede-iglesias/shipkit/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> [!WARNING]
> En desarrollo temprano. La API se mueve hasta el primer tag estable (`v1`).

`shipkit` es la base reutilizable para construir CLIs/TUIs en Go con un ciclo de
vida self-contained: el mismo binario se instala, se actualiza desde GitHub
releases (repos privados o públicos, vía la sesión de `gh`), se diagnostica y se
desinstala solo. Encima trae un kit de componentes de terminal listos para
componer.

## Arquitectura en capas

La dependencia apunta **siempre hacia abajo**. La base (los sub-paquetes de `ui`
y `theme`) es agnóstica de todo lo que tiene encima; las pantallas y el lifecycle
componen la base. Un widget de `ui` nunca importa `lifecycle` ni `screens`.

```
app/         struct App{} + Build()    ← tu app configura acá
screens/     menu · dashboard · logs   ← TUIs que componen lo de abajo
lifecycle/   install · update · uninstall · doctor · clean
ui/ + theme/ base: widgets agnósticos de paleta + estilos
```

## Instalación

```sh
go get github.com/fede-iglesias/shipkit
```

## Quick start

```go
// cmd/miapp/main.go
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/fede-iglesias/shipkit"
	"github.com/fede-iglesias/shipkit/app"
)

var version = "dev" // inyectado con -ldflags en el build

func main() {
	a := app.App{
		Name:       "miapp",
		BinaryName: "miapp",
		Repo:       "tu-usuario/miapp",
		Version:    version,
	}
	// install / update / uninstall / doctor / menu vienen gratis.
	root := shipkit.Build(a)
	_ = fang.Execute(context.Background(), root)
	os.Exit(0)
}
```

## Licencia

[MIT](LICENSE) © 2026 Fede Iglesias
