CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE district (
    id          serial PRIMARY KEY,
    adcode      char(6)        NOT NULL,
    name        varchar(32)    NOT NULL,
    center      geometry(Point, 4326)        NOT NULL,
    geom        geometry(MultiPolygon, 4326) NOT NULL,
    area_km2    numeric(10, 2) NOT NULL CHECK (area_km2 > 0),
    created_at  timestamptz    NOT NULL DEFAULT now(),
    updated_at  timestamptz    NOT NULL DEFAULT now(),
    CONSTRAINT district_adcode_uk   UNIQUE (adcode),
    CONSTRAINT district_adcode_fmt  CHECK (adcode ~ '^[0-9]{6}$'),
    CONSTRAINT district_geom_valid  CHECK (ST_IsValid(geom))
);

CREATE INDEX district_geom_gist ON district USING gist (geom);

CREATE TABLE poi_category (
    id          smallserial PRIMARY KEY,
    code        varchar(32) NOT NULL,
    name        varchar(32) NOT NULL,
    color       char(7)     NOT NULL,
    sort_order  smallint    NOT NULL DEFAULT 0,
    CONSTRAINT poi_category_code_uk    UNIQUE (code),
    CONSTRAINT poi_category_color_fmt  CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);

CREATE TABLE poi (
    id           bigserial    PRIMARY KEY,
    osm_type     char(1)      NOT NULL,
    osm_id       bigint       NOT NULL,
    name         varchar(128) NOT NULL,
    name_en      varchar(128),
    category_id  smallint     NOT NULL REFERENCES poi_category (id),
    district_id  integer      REFERENCES district (id) ON DELETE SET NULL,
    geom         geometry(Point, 4326) NOT NULL,
    address      text,
    tags         jsonb        NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    CONSTRAINT poi_osm_uk         UNIQUE (osm_type, osm_id),
    CONSTRAINT poi_osm_type_chk   CHECK (osm_type IN ('n', 'w', 'r')),
    CONSTRAINT poi_name_not_blank CHECK (btrim(name) <> '')
);

CREATE INDEX poi_geom_gist        ON poi USING gist (geom);
CREATE INDEX poi_geog_gist        ON poi USING gist ((geom::geography));
CREATE INDEX poi_name_trgm        ON poi USING gin (name gin_trgm_ops);
CREATE INDEX poi_category_id_idx  ON poi (category_id);
CREATE INDEX poi_district_id_idx  ON poi (district_id);
