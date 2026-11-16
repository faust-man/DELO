CREATE TABLE IF NOT EXISTS branches(id uuid NOT NULL,org uuid NOT NULL REFERENCES organizations,name text NOT NULL,address text NOT NULL,timezone text NOT NULL,active boolean NOT NULL DEFAULT true,PRIMARY KEY(org,id));
CREATE TABLE IF NOT EXISTS rooms(id uuid NOT NULL,org uuid NOT NULL,branch_id uuid NOT NULL,name text NOT NULL,capacity integer NOT NULL CHECK(capacity>0),PRIMARY KEY(org,id),FOREIGN KEY(org,branch_id) REFERENCES branches(org,id));
CREATE TABLE IF NOT EXISTS groups(id uuid NOT NULL,org uuid NOT NULL REFERENCES organizations,name text NOT NULL,PRIMARY KEY(org,id));
ALTER TABLE classes ADD COLUMN IF NOT EXISTS ends timestamptz;
ALTER TABLE classes ADD COLUMN IF NOT EXISTS room_id uuid;
ALTER TABLE classes ADD COLUMN IF NOT EXISTS teacher_id uuid;
ALTER TABLE classes ADD COLUMN IF NOT EXISTS group_id uuid;
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='classes_room_fk' AND conrelid='classes'::regclass) THEN
 ALTER TABLE classes ADD CONSTRAINT classes_room_fk FOREIGN KEY(org,room_id) REFERENCES rooms(org,id);
 ALTER TABLE classes ADD CONSTRAINT classes_teacher_fk FOREIGN KEY(org,teacher_id) REFERENCES memberships(org,user_id);
 ALTER TABLE classes ADD CONSTRAINT classes_group_fk FOREIGN KEY(org,group_id) REFERENCES groups(org,id);
 ALTER TABLE classes ADD CONSTRAINT classes_interval CHECK(ends IS NULL OR ends>starts);
 END IF;
END $$;
CREATE TABLE IF NOT EXISTS class_changes(id uuid NOT NULL,org uuid NOT NULL,class_id uuid NOT NULL,old_starts timestamptz NOT NULL,old_ends timestamptz,new_starts timestamptz NOT NULL,new_ends timestamptz,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(org,id),FOREIGN KEY(org,class_id) REFERENCES classes(org,id));
CREATE TABLE IF NOT EXISTS operational_tasks(id uuid NOT NULL,org uuid NOT NULL REFERENCES organizations,object_id uuid NOT NULL,reason text NOT NULL,state text NOT NULL DEFAULT 'open',created_at timestamptz NOT NULL DEFAULT now(),PRIMARY KEY(org,id));
