#[derive(Debug, Clone)]
pub enum InstructionParam<TCtx> {
    String(String),
    Func(fn(&TCtx) -> String),
}

impl<TCtx> InstructionParam<TCtx> {
    pub fn as_string(&self, context: &TCtx) -> String {
        match self {
            Self::String(s) => s.clone(),
            Self::Func(f) => f(context),
        }
    }
}

pub fn get_prompt<TCtx>(instructions: &[InstructionParam<TCtx>], context: &TCtx) -> String {
    instructions
        .iter()
        .map(|param| param.as_string(context))
        .collect::<Vec<_>>()
        .join("\n")
}
