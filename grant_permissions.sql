-- Grant permissions to goera_user
GRANT ALL ON SCHEMA public TO goera_user;
GRANT ALL ON ALL TABLES IN SCHEMA public TO goera_user;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO goera_user;
ALTER USER goera_user CREATEDB; 