-- 009_user_role_and_invitation.down.sql
DROP TABLE IF EXISTS invitation;
ALTER TABLE "user" DROP COLUMN IF EXISTS role;
