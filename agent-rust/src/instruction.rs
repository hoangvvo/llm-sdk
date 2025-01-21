pub enum InstructionParam<TCtx> {
    String(String),
    Func(Box<dyn Fn(&TCtx) -> String + Send + Sync>),
}

impl<TCtx> InstructionParam<TCtx> {
    pub fn as_string(&self, context: &TCtx) -> String {
        match self {
            Self::String(s) => s.clone(),
            Self::Func(f) => f(context),
        }
    }
}

impl<TCtx> From<String> for InstructionParam<TCtx> {
    fn from(value: String) -> Self {
        Self::String(value)
    }
}

impl<TCtx> From<&str> for InstructionParam<TCtx> {
    fn from(value: &str) -> Self {
        Self::String(value.to_string())
    }
}

impl<TCtx, F> From<F> for InstructionParam<TCtx>
where
    F: Fn(&TCtx) -> String + Send + Sync + 'static,
    TCtx: Send + Sync + 'static,
{
    fn from(value: F) -> Self {
        Self::Func(Box::new(value))
    }
}

pub fn get_prompt<TCtx>(instructions: &[InstructionParam<TCtx>], context: &TCtx) -> String {
    instructions
        .iter()
        .map(|param| param.as_string(context))
        .collect::<Vec<_>>()
        .join("\n")
}
