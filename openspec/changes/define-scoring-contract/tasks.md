## 1. Entrada normalizada del contrato

- [ ] 1.1 Crear `internal/scoring/doc.go` declarando el alcance del paquete —contrato, no
  algoritmo—, la prohibición de la épica LAB-17 y quién lo consumirá (LAB-33); verificar
  que el paquete no importa `internal/employee` ni `internal/jobposition` con
  `go list -deps ./internal/scoring | grep -E 'internal/(employee|jobposition)'` sin
  coincidencias.
- [ ] 1.2 Crear `internal/scoring/levels.go` con los tipos `ExperienceLevel` y
  `EducationLevel`, sus constantes con los mismos valores string que los `oneof` de
  `employee` y `jobposition`, un método `Valid()` y un orden explícito entre niveles;
  verificar con un test table-driven que cubre valores válidos e inválidos.
- [ ] 1.3 Agregar en `internal/scoring/levels_test.go` un test de contrato que fija los
  valores string esperados de ambas enumeraciones, para que una divergencia con
  `employee` o `jobposition` rompa la suite en vez de producir pares que nunca matchean.
- [ ] 1.4 Crear `internal/scoring/models.go` con `EmployeeProfile` y `JobRequirements`
  conteniendo los cinco atributos comparables (experiencia, nivel educativo, horas
  diarias, timezone, recursos técnicos) más sus identificadores, usando punteros para los
  atributos que admiten ausencia; verificar con un test que la ausencia es distinguible
  del valor cero.
- [ ] 1.5 Agregar el tipo `Pair` que une un `EmployeeProfile` y un `JobRequirements`, con
  un accesor a ambos identificadores; verificar con un test que un `Pair` conserva los
  identificadores necesarios para construir un `recommendation.Candidate` sin correlación
  adicional.

## 2. Resultado e indicadores

- [ ] 2.1 Agregar en `models.go` los tipos `Indicator` (nombre, peso, valor) y
  `Eligibility` (elegible más motivo); verificar que compilan y que el motivo solo es
  significativo cuando el par es inelegible, con un test del constructor correspondiente.
- [ ] 2.2 Agregar el tipo `Result` con `EmployeeID`, `JobPositionID`, `Eligible`,
  `Reason`, `Total *float64` e `Indicators []Indicator`; verificar con un test
  table-driven que los tres desenlaces —puntuado, inelegible, elegible sin puntaje— son
  mutuamente distinguibles y que puntaje cero difiere de puntaje ausente.
- [ ] 2.3 Agregar constructores `Scored`, `Unscored` y `Rejected` que impidan armar un
  `Result` incoherente (por ejemplo, inelegible con puntaje); verificar con un test que
  cada constructor produce exactamente el desenlace esperado.
- [ ] 2.4 Agregar un test que construye un `Result` con indicadores y comprueba que
  mapear a `recommendation.Candidate` transporta solo `Total`, dejando los indicadores
  fuera del esquema principal.

## 3. Puertos

- [ ] 3.1 Crear `internal/scoring/scorer.go` con la interfaz `HardFilter`
  (`Eligible(ctx, Pair) (Eligibility, error)`), documentando que no produce puntaje;
  verificar que compila y que el doc comment lo declara explícitamente.
- [ ] 3.2 Agregar en el mismo archivo la interfaz `Scorer` con `Score(ctx, Pair)
  (Result, error)` y `ScoreAll(ctx, []Pair) ([]Result, error)`, documentando la
  obligación de correspondencia uno a uno, orden preservado y equivalencia con `Score`;
  verificar que compila.
- [ ] 3.3 Crear `internal/scoring/errors.go` con `ErrScoringUnavailable` y los errores
  sentinela de entrada inválida necesarios para que un llamador distinga dependencia no
  disponible de error de entrada; verificar con un test que `errors.Is` los clasifica por
  separado.

## 4. Implementación de producción no disponible

- [ ] 4.1 Crear `internal/scoring/unavailable.go` con el tipo `Unavailable` que implementa
  `Scorer` y `HardFilter` y devuelve siempre `ErrScoringUnavailable`; verificar con
  aserciones de interfaz en tiempo de compilación (`var _ Scorer = Unavailable{}`).
- [ ] 4.2 Agregar un test que cubre `Score`, `ScoreAll` y `Eligible` de `Unavailable`
  comprobando que ninguna devuelve puntaje, que `ScoreAll` no devuelve resultados
  parciales y que el error satisface `errors.Is(err, ErrScoringUnavailable)`.
- [ ] 4.3 Verificar con `grep -rn "scoring" cmd/` que la composición de dependencias no
  referencia el paquete: el cableado pertenece a LAB-33.

## 5. Fake determinista de tests

- [ ] 5.1 Crear `internal/scoring/fake_test.go` con un fake configurable que implementa
  ambas interfaces y puede producir los cuatro desenlaces —puntuado, inelegible, sin
  puntaje y error—; verificar que el archivo es `_test.go` y por lo tanto no forma parte
  del binario.
- [ ] 5.2 Agregar un test de reproducibilidad: dos evaluaciones del mismo par con la misma
  configuración devuelven resultados idénticos.
- [ ] 5.3 Agregar un test de contrato sobre el fake que ejercita la equivalencia entre
  `Score` y `ScoreAll`, el lote vacío y el lote mixto de pares elegibles e inelegibles,
  documentándolo como el test que cualquier implementación futura debe pasar.

## 6. Validación y cierre

- [ ] 6.1 Ejecutar `gofmt -l internal/scoring` sin salida y `go vet ./...` sin hallazgos.
- [ ] 6.2 Ejecutar `go test ./internal/scoring` y luego `go test ./...`, ambos en verde.
- [ ] 6.3 Recorrer los cuatro criterios de aceptación de LAB-30 y dejar constancia de con
  qué test o comprobación se satisface cada uno.
- [ ] 6.4 Actualizar la sección de arquitectura de `CLAUDE.md` y `AGENTS.md` del backend
  para listar `internal/scoring` como transversal, manteniendo ambos archivos alineados.
