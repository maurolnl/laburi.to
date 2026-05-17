-- name: CreateRefreshToken :one
insert into refresh_token(token, created_at, updated_at, user_id, expires_at, revoked_at)
values(
$1,
$2,
$3,
$4,
$5,
$6
)
returning *;

-- name: GetActiveRefreshToken :one
select * from refresh_token
where token = $1
  and revoked_at is null
  and expires_at > now();

-- name: GetRefreshToken :one
select * from refresh_token
where token = $1;

-- name: RevokeRefreshToken :exec
update refresh_token set revoked_at = now(), updated_at = now() where refresh_token.token = $1;

-- name: GetUserFromRefreshToken :one
select users.* from refresh_token inner join users on users.id = refresh_token.user_id where refresh_token.token = $1;
