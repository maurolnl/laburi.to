## MODIFIED Requirements

### Requirement: Fallo temprano con mensajes seguros
Todo error de configuración del transporte SHALL producirse durante el arranque y antes de
atender la primera petición, y MUST ser clasificable por el llamador sin inspeccionar el
texto del error. El mensaje MUST nombrar la variable responsable y MUST NOT contener su
valor, credenciales, identificadores de cuenta ni la ubicación completa de ninguna cola.

Los interruptores de habilitación quedan fuera de esa regla: un valor que no pueda
interpretarse MUST resolverse como deshabilitado y MUST NOT impedir el arranque, porque
describen si una funcionalidad participa y no cómo participa. Con el transporte habilitado,
en cambio, toda variable obligatoria ausente y todo valor numérico ilegible o fuera de rango
MUST seguir abortando el arranque.

#### Scenario: Fallo antes de atender tráfico
- **WHEN** la configuración de un transporte habilitado es inválida
- **THEN** el proceso termina durante el arranque y no llega a atender ninguna petición
  HTTP

#### Scenario: Interruptor ilegible sin abortar
- **WHEN** un interruptor de habilitación tiene un valor que no puede interpretarse
- **THEN** la funcionalidad queda deshabilitada, el proceso arranca con normalidad y atiende
  su contrato HTTP sin cambios

#### Scenario: Mensaje sin secretos
- **WHEN** se emite un error o una advertencia de configuración
- **THEN** el mensaje identifica la variable responsable y no incluye su valor ni ningún
  dato sensible

#### Scenario: Clasificación del error
- **WHEN** un llamador recibe un error de configuración
- **THEN** puede distinguir un valor faltante de un valor inválido sin comparar cadenas de
  texto

#### Scenario: Diagnóstico sin exponer la cola
- **WHEN** el arranque registra el estado del transporte
- **THEN** el registro indica si está habilitado y no revela la ubicación de la cola ni
  credenciales

### Requirement: Transporte deshabilitado como estado legítimo
El transporte SHALL poder quedar deshabilitado, y en ese estado la aplicación MUST arrancar
con normalidad y MUST conservar sin cambios todo su contrato HTTP. Con el transporte
deshabilitado, el acceso MUST seguir estando disponible como valor no nulo y toda operación
sobre él MUST fallar de forma explícita e identificable, nunca simular éxito ni perder un
mensaje en silencio.

Se llega a ese estado por ausencia, por un valor vacío o por un valor que no pueda
interpretarse. Este último MUST producir una advertencia que nombre la variable responsable,
de modo que un interruptor mal escrito no quede indistinguible de uno apagado a propósito.

#### Scenario: Arranque sin cola configurada
- **WHEN** el transporte no está habilitado y no se declara ninguna de sus variables
- **THEN** la aplicación arranca y responde su contrato HTTP actual sin cambios

#### Scenario: Apagado por valor ilegible con advertencia
- **WHEN** el interruptor del transporte o el del consumidor tiene un valor que no puede
  interpretarse
- **THEN** la resolución de la configuración informa una advertencia que nombra esa variable
  y el arranque la registra

#### Scenario: Apagado deliberado sin advertencia
- **WHEN** un interruptor está ausente, vacío o declarado como falso
- **THEN** no se informa ninguna advertencia, porque no hay nada que corregir

#### Scenario: Operación sobre un transporte deshabilitado
- **WHEN** un consumidor intenta usar el transporte deshabilitado
- **THEN** la operación falla con un error identificable como "transporte no habilitado" y
  no reporta éxito

#### Scenario: Ausencia de valor nulo
- **WHEN** un consumidor recibe el transporte deshabilitado
- **THEN** recibe un valor utilizable y no un valor nulo que deba comprobar antes de cada
  llamada
