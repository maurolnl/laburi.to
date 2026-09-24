package employee

import (
	"context"
	"errors"

	"github.com/maurolnl/bolsa-de-trabajo-back/internal/auth"
	"github.com/maurolnl/bolsa-de-trabajo-back/internal/user"
)

// profileViewer es lo que la autorización devuelve además del permiso: si quien lee es el dueño
// del perfil. El dato no es decorativo, decide si el correo del empleado viaja en la respuesta.
type profileViewer struct {
	isOwner bool
}

// authorizeProfileAccess resuelve quién puede leer un perfil ajeno y quién no. Es la única
// definición de esa regla en el paquete: el perfil y las dos rutas de entrega la invocan, de
// modo que la entrega no pueda convertirse en la puerta de atrás del perfil.
//
// La identidad sale siempre del JWT. El employeeID del path solo sirve para detectar que
// alguien pidió un perfil que no le corresponde, nunca para resolver quién es.
//
// Toda falla devuelve ErrProfileAccessForbidden, incluido el empleado inexistente. Es
// deliberado y se aparta de lo que hace el borde de consulta de recomendaciones: allá el sujeto
// del path es siempre propio, así que un 404 solo lo ve quien ya conoce sus identificadores.
// Acá el empleador recorre identificadores ajenos por definición, y distinguir «no existe» de
// «no te corresponde» le permitiría enumerar qué empleados hay en la plataforma.
func (s *employeeService) authorizeProfileAccess(ctx context.Context, employeeID int32, principal auth.Principal) (profileViewer, error) {
	switch principal.Role {
	case user.UserRoleEmployee:
		return s.authorizeOwnProfile(ctx, employeeID, principal)
	case user.UserRoleEmployer:
		return s.authorizeRecommendedProfile(ctx, employeeID, principal)
	default:
		return profileViewer{}, ErrProfileAccessForbidden
	}
}

// authorizeOwnProfile compara el dueño del perfil contra el usuario del token. Resuelve la
// propiedad con la lectura mínima —identificador y usuario— y no con el perfil entero, para que
// las tres rutas autoricen igual: dos de ellas no necesitan el perfil para nada.
//
// El empleado inexistente cae en el mismo error que el perfil ajeno.
func (s *employeeService) authorizeOwnProfile(ctx context.Context, employeeID int32, principal auth.Principal) (profileViewer, error) {
	owner, err := s.repo.GetEmployeeByID(ctx, employeeID)
	if errors.Is(err, ErrEmployeeProfileNotFound) {
		return profileViewer{}, ErrProfileAccessForbidden
	}
	if err != nil {
		return profileViewer{}, err
	}

	if owner.UserID != principal.UserID {
		return profileViewer{}, ErrProfileAccessForbidden
	}

	return profileViewer{isOwner: true}, nil
}

// authorizeRecommendedProfile delega la regla en el puerto de recomendaciones, que responde por
// sí o por no. No consulta antes si el empleado existe: el puerto ya devuelve false para un
// empleado inexistente, y preguntarlo aparte reintroduciría la distinción que este diseño evita.
//
// Un empleador que todavía no creó su perfil no necesita caso propio: no puede ser dueño de
// ningún puesto, así que ningún vínculo lo alcanza.
func (s *employeeService) authorizeRecommendedProfile(ctx context.Context, employeeID int32, principal auth.Principal) (profileViewer, error) {
	allowed, err := s.access.EmployerHasCurrentRecommendation(ctx, employeeID, principal.UserID)
	if err != nil {
		return profileViewer{}, err
	}
	if !allowed {
		return profileViewer{}, ErrProfileAccessForbidden
	}

	return profileViewer{isOwner: false}, nil
}
