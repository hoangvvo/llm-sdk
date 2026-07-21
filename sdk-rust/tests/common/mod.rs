pub mod cases;
#[allow(dead_code)]
pub mod transports;

pub fn install_tls_provider() {
    use std::sync::Once;

    static INIT: Once = Once::new();
    INIT.call_once(|| {
        if rustls::crypto::CryptoProvider::get_default().is_none() {
            rustls::crypto::ring::default_provider()
                .install_default()
                .expect("the test application must select its Rustls provider once");
        }
    });
}
