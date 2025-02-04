use crate::{Part, SourcePart};

pub(crate) fn get_compatible_parts_without_source_parts(parts: Vec<Part>) -> Vec<Part> {
    parts
        .into_iter()
        .flat_map(|part| match part {
            Part::Source(SourcePart { content, .. }) => {
                get_compatible_parts_without_source_parts(content)
            }
            _ => vec![part],
        })
        .collect()
}
