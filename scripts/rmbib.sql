DELIMITER //

CREATE FUNCTION rm_corpus_bibinfo(p_corpus_name VARCHAR(63))
RETURNS INT
DETERMINISTIC
BEGIN
    -- Store the values we'll need to remove
    DECLARE v_bib_id_struct, v_bib_id_attr, v_bib_label_struct, v_bib_label_attr VARCHAR(63);

    -- Get the current values
    SELECT bib_id_struct, bib_id_attr, bib_label_struct, bib_label_attr
    INTO v_bib_id_struct, v_bib_id_attr, v_bib_label_struct, v_bib_label_attr
    FROM kontext_corpus
    WHERE name = p_corpus_name;

    -- Clear the fields in kontext_corpus
    UPDATE kontext_corpus
    SET bib_id_struct = NULL,
        bib_id_attr = NULL,
        bib_label_struct = NULL,
        bib_label_attr = NULL
    WHERE name = p_corpus_name;

    -- Remove the attributes from corpus_structattr
    DELETE FROM corpus_structattr
    WHERE corpus_name = p_corpus_name
    AND (
        (structure_name = v_bib_id_struct AND name = v_bib_id_attr)
        OR
        (structure_name = v_bib_label_struct AND name = v_bib_label_attr)
    );

    -- Remove the structures from corpus_structure if they're not used by other attributes
    DELETE FROM corpus_structure
    WHERE corpus_name = p_corpus_name
    AND name IN (v_bib_id_struct, v_bib_label_struct)
    AND NOT EXISTS (
        SELECT 1 FROM corpus_structattr
        WHERE corpus_name = p_corpus_name
        AND structure_name = corpus_structure.name
    );

    RETURN 1;
END //

DELIMITER ;