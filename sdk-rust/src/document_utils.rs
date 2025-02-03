use crate::{DocumentPart, Part};

pub(crate) fn get_compatible_parts_without_document_parts(parts: Vec<Part>) -> Vec<Part> {
    parts
        .into_iter()
        .flat_map(|part| match part {
            Part::Document(DocumentPart { content, .. }) => {
                get_compatible_parts_without_document_parts(content)
            }
            _ => vec![part],
        })
        .collect()
}
