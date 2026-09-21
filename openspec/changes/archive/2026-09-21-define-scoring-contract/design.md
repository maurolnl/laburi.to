## Context

Ver `proposal.md` — Why para la motivación.

Estado actual relevante:

- `internal/recommendation` (LAB-29) ya persiste batches y recomendaciones. Su
  `Candidate` expone `Score *float64` y `store.go` declara `CompleteBatch(ctx, batchID,
  []Candidate)`. El puntaje ya es opcional en la base (`recommendations.score NUMERIC
  NULL`) y en el modelo Go.
- `internal/jobposition` ya estableció el precedente de puerto sin tecnología:
  `JobPositionEventPublisher` con una `NoopEventPublisher` vigente mientras la épica de
  recomendaciones no existe. Este cambio sigue esa forma, con una diferencia deliberada:
  el equivalente de scoring **no** puede ser un noop silencioso.
- Los atributos comparables ya existen en ambos lados y son enumeraciones cerradas en
  `internal/employee` y `internal/jobposition`: experiencia (`less_1y` … `more_10y`),
  nivel educativo (`university`, `postgraduate`, `high-school-orientation`, `tertiary`),
  horas diarias (1–8), timezone y una lista de recursos técnicos.
- La convención del repositorio es `internal/<feature>` con `store.go` para interfaces
  del lado consumidor y `models.go` para tipos de dominio.

Restricción dura de la épica LAB-17: no definir ni simular el algoritmo. Este cambio
llega hasta el borde de esa restricción y no la cruza.

## Goals / Non-Goals

**Goals:**

- Un puerto que LAB-33 pueda inyectar y testear hoy, con un algoritmo que no existe.
- Un tipo de resultado que sobreviva a la aparición del algoritmo real sin cambiar el
  transporte ni el esquema de `recommendations`.
- Un modo de fallo que el worker pueda traducir a `failed` sin ambigüedad.

**Non-Goals:**

- Elegir indicadores, pesos o función de combinación.
- Elegir qué filtros duros aplica el producto (el contrato define el puerto, no sus
  reglas).
- Definir cómo el worker arma la entrada normalizada desde la base: eso es LAB-33.
- Cablear nada en `cmd/api.go`.
- Cualquier cambio en `internal/recommendation`.

## Decisions

### Paquete propio `internal/scoring`, sin importar `employee` ni `jobposition`

El contrato define sus propios tipos de entrada (`EmployeeProfile`, `JobRequirements`) en
vez de reutilizar `employee.Employee` y `jobposition.JobPosition`.

Por qué: `employee.Employee` arrastra email, archivos, portfolio y educación completa —
datos que el scoring no compara y que lo atarían al modelo de persistencia del perfil.
Un tipo propio y chico deja explícito qué se compara y hace que el paquete sea testeable
sin construir un perfil entero.

Alternativas descartadas:

- *Contrato dentro de `internal/recommendation`*: mezcla persistencia y cálculo en un
  paquete cuyo doc.go declara alcance de persistencia, y obliga a `recommendation` a
  conocer atributos de perfil que hoy no conoce.
- *Recibir los modelos de dominio directamente*: crea una dependencia de `scoring` hacia
  dos paquetes de feature y un ciclo latente cuando LAB-34 haga que esos paquetes
  disparen regeneración.

Costo aceptado: hace falta un mapeo explícito de dominio a entrada normalizada. Ese
mapeo pertenece a LAB-33, que es quien lee de la base.

### Enumeraciones redeclaradas como tipos del paquete

`ExperienceLevel` y `EducationLevel` se redeclaran en `scoring` con los mismos valores
string que ya usan los `validate:"oneof=..."` de ambos paquetes.

Por qué: mantiene la independencia del punto anterior y permite que el contrato exponga
un orden entre niveles, que es lo que un comparador necesita y lo que hoy ningún paquete
ofrece. Los valores string idénticos hacen que el mapeo de LAB-33 sea una conversión
directa y verificable.

Riesgo de divergencia: se mitiga con un test que fija los valores esperados, de modo que
cambiar una enumeración en `employee` o `jobposition` sin actualizar `scoring` rompa la
suite en vez de producir un par que nunca matchea.

### Ausencia de valor con punteros, no con el valor cero

Los atributos opcionales de la entrada (horas disponibles, nivel educativo del empleado)
se representan con punteros.

Por qué: el mismo criterio que LAB-29 aplicó a `Candidate.Score`. Cero horas y horas
desconocidas son estados distintos, y un algoritmo futuro puede querer penalizar uno y no
el otro. Colapsarlos ahora cierra esa puerta en silencio.

### `Result` con total opcional más indicadores, en vez de un `float64` pelado

```
type Indicator struct {
    Name   string
    Weight float64
    Value  float64
}

type Result struct {
    EmployeeID    int32
    JobPositionID int32
    Eligible      bool
    Reason        string      // solo cuando Eligible es false
    Total         *float64    // nil = elegible pero sin puntaje disponible
    Indicators    []Indicator // detalle, puede estar vacío
}
```

Por qué: es lo que hace cierto el criterio de aceptación "incorporar indicadores
parciales/totales sin cambiar transporte ni esquema". Los indicadores viven solo en
memoria; a `recommendation.Candidate` viaja únicamente `Total`, que es exactamente el
`*float64` que ya espera. Agregar un indicador no toca la base ni el transporte.

Alternativa descartada: un `map[string]float64`. Pierde el orden, pierde el peso y no
tiene un lugar natural para documentar cada indicador.

Sobre `Reason`: es un string libre y no una enumeración porque los filtros duros del
producto todavía no están definidos. Enumerarlos ahora sería inventar reglas que el
ticket prohíbe.

### Tres desenlaces en un único tipo, no tres tipos

`Eligible=false` (descartado), `Total=nil` con `Eligible=true` (sin puntaje) y `Total`
presente (puntuado) conviven en `Result`.

Por qué: el worker recorre una lista homogénea y decide por par. Tres tipos distintos lo
obligarían a un type switch sin ganancia.

### `Scorer` con `Score` y `ScoreAll`; `HardFilter` aparte

```
type HardFilter interface {
    Eligible(ctx context.Context, pair Pair) (Eligibility, error)
}

type Scorer interface {
    Score(ctx context.Context, pair Pair) (Result, error)
    ScoreAll(ctx context.Context, pairs []Pair) ([]Result, error)
}
```

Por qué dos interfaces: es la separación que pide el ticket. Un filtro duro es una
decisión booleana barata que puede aplicarse antes de traer datos caros; el cálculo es
posterior y costoso. Fusionarlos obligaría a toda implementación de filtro a fingir un
puntaje.

Por qué `ScoreAll` además de `Score`: un batch puntúa N pares contra el mismo sujeto. Una
implementación con costo fijo por invocación —un índice, un modelo, una llamada remota—
no puede amortizarlo si el contrato solo ofrece el par. El contrato obliga a que
`ScoreAll` coincida par por par con `Score`, así que una implementación trivial puede ser
un loop y una optimizada puede batchear, sin que LAB-33 note la diferencia.

Alternativa descartada: `ScoreSubject(ctx, subject, []Candidate)`. Es más eficiente en el
papel, pero se aparta del "par employee-job" del ticket y fuerza dos firmas según el
sentido del batch.

### Implementación de producción que falla, no que devuelve cero

`Unavailable` implementa `Scorer` y `HardFilter` y devuelve siempre
`ErrScoringUnavailable`.

Por qué falla en lugar de devolver `Total=nil`: `Total=nil` es un desenlace legítimo que
significa "este par no tiene puntaje", y un batch con todos los pares sin puntaje se
completaría como `completed` y se mostraría al usuario como una recomendación real sin
ordenar. La épica pide lo contrario: sin algoritmo, la generación está bloqueada. El
error hace que LAB-33 transicione el batch a `failed`, que es el estado que la épica
reserva para esto.

Por qué no un noop al estilo `NoopEventPublisher`: ese noop es correcto porque un evento
perdido no corrompe nada. Un puntaje inventado sí.

`ScoreAll` falla entera y no devuelve resultados parciales: un lote a medias tentaría a
un llamador a completar el batch con lo que llegó.

### Fake en `_test.go`, no en el paquete

El fake determinista vive en `scoring/fake_test.go` y es `export_test`-style: accesible
desde los tests del propio paquete. Los consumidores de otros paquetes (LAB-33) escriben
su propio doble contra la interfaz, que es lo que ya hace `employee/fakes_test.go`.

Por qué: cumple literalmente "fake solo para tests, nunca algoritmo temporal en
producción" a nivel de compilación, no de disciplina. Un `scoring.NewFake()` exportado en
un archivo normal sería seleccionable desde `cmd/api.go` por error.

Costo aceptado: LAB-33 duplica unas líneas de doble. Es preferible a una puerta abierta.

### Sin cableado en `cmd/api.go`

No hay consumidor: el worker es LAB-33. Inyectar `Unavailable{}` hoy agregaría un campo
muerto a la composición.

## Risks / Trade-offs

- **La entrada normalizada se define sin conocer el algoritmo, y puede faltarle un
  atributo** → Los cinco atributos elegidos son los que ambos lados ya declaran y
  comparan (`required_experience` ↔ `years_of_experience`, `required_education_level` ↔
  educación, horas, timezone, recursos). Agregar un campo a un struct de entrada es un
  cambio local y compatible; el spec no fija la lista como cerrada.
- **Las enumeraciones redeclaradas pueden divergir de `employee` / `jobposition`** → Test
  que fija los valores esperados; una divergencia rompe la suite.
- **`ScoreAll` y `Score` pueden divergir en una implementación futura** → El spec exige
  equivalencia y hay un test de contrato sobre el fake que la verifica. LAB-33 debería
  correr el mismo test contra cualquier implementación nueva.
- **Nada consume el contrato en este ticket, así que podría envejecer mal antes de
  LAB-33** → Mitigado por el alcance chico: son tipos y dos interfaces. Si LAB-33
  descubre que la forma no sirve, corregirla es barato porque no hay persistencia ni
  transporte atados.
- **`Unavailable` hace que cualquier ejecución real termine en `failed`** → Es el
  comportamiento pedido por la épica, no una regresión. Queda registrado en el proposal
  que el ticket del algoritmo productivo sigue sin existir.

## Migration Plan

No aplica: código nuevo, sin migraciones de base de datos, sin cambios de contrato HTTP y
sin consumidores existentes. Revertir es borrar el paquete.
