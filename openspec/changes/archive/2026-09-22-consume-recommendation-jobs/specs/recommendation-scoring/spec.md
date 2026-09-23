## ADDED Requirements

### Requirement: Comprobación previa y opcional de disponibilidad
El contrato de scoring SHALL admitir que una implementación declare por adelantado que no va
a poder evaluar, sin necesidad de invocarla sobre un lote. Esa comprobación MUST ser opcional
para la implementación —una que siempre pueda evaluar no está obligada a ofrecerla— y su
resultado negativo MUST ser el mismo fallo identificable de dependencia no disponible que
produce la evaluación, para que el llamador lo clasifique igual.

La implementación de producción, mientras el algoritmo de indicadores no exista, MUST
declararse no disponible en esa comprobación.

#### Scenario: Comprobación en la implementación de producción
- **WHEN** se comprueba la disponibilidad de la implementación de producción
- **THEN** informa el fallo de dependencia no disponible, sin necesidad de evaluar ningún
  par

#### Scenario: Implementación sin comprobación
- **WHEN** una implementación no ofrece la comprobación previa
- **THEN** el llamador la considera disponible y procede a evaluar

#### Scenario: Mismo fallo que la evaluación
- **WHEN** un llamador recibe el resultado negativo de la comprobación previa
- **THEN** puede clasificarlo como dependencia no disponible con el mismo criterio que
  aplica al fallo de la evaluación

#### Scenario: Decisión previa a abrir trabajo
- **WHEN** un consumidor necesita saber si puede puntuar antes de marcar un trabajo como en
  ejecución
- **THEN** la comprobación se lo permite sin efectos sobre ningún par ni sobre ningún estado
  persistido
