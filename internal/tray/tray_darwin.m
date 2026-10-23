//go:build darwin

#import <Cocoa/Cocoa.h>

extern void goOnToggleMaster(void);
extern void goOnToggleAccount(int index);
extern void goOnLinkAccount(void);
extern void goOnOpenConfig(void);
extern void goOnQuit(void);

@interface MacTrayHandler : NSObject
@property (strong, nonatomic) NSStatusItem *statusItem;
@property (copy, nonatomic) NSString *iconPath;
@end

static MacTrayHandler *g_trayHandler = nil;

@implementation MacTrayHandler

- (void)actionToggleMaster:(id)sender {
    goOnToggleMaster();
}

- (void)actionToggleAccount:(id)sender {
    NSMenuItem *item = (NSMenuItem *)sender;
    goOnToggleAccount((int)item.tag);
}

- (void)actionLinkAccount:(id)sender {
    goOnLinkAccount();
}

- (void)actionOpenConfig:(id)sender {
    goOnOpenConfig();
}

- (void)actionQuit:(id)sender {
    goOnQuit();
}

- (void)setupMenuWithMaster:(BOOL)masterActive accounts:(NSArray *)accounts {
    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"CloudflareNetConnect"];

    // 1. Cabecera informativa
    NSMenuItem *header = [[NSMenuItem alloc] initWithTitle:@"CloudflareNetConnect v1.0.0" action:nil keyEquivalent:@""];
    [header setEnabled:NO];
    [menu addItem:header];
    [menu addItem:[NSMenuItem separatorItem]];

    // 2. Interruptor Maestro
    NSString *masterTitle = masterActive ? @"⚡ Servicio de Túneles (Activo)" : @"⏸ Servicio de Túneles (Pausado)";
    NSMenuItem *masterItem = [[NSMenuItem alloc] initWithTitle:masterTitle action:@selector(actionToggleMaster:) keyEquivalent:@""];
    [masterItem setTarget:self];
    [masterItem setState:masterActive ? NSControlStateValueOn : NSControlStateValueOff];
    [menu addItem:masterItem];
    [menu addItem:[NSMenuItem separatorItem]];

    // 3. Sección de Cuentas
    if (accounts.count == 0) {
        NSMenuItem *noAcc = [[NSMenuItem alloc] initWithTitle:@"(Sin cuentas vinculadas)" action:nil keyEquivalent:@""];
        [noAcc setEnabled:NO];
        [menu addItem:noAcc];
    } else {
        for (int i = 0; i < (int)accounts.count; i++) {
            NSDictionary *acc = accounts[i];
            NSString *name = acc[@"name"];
            BOOL enabled = [acc[@"enabled"] boolValue];
            NSString *title = [NSString stringWithFormat:@"%@ %@", enabled ? @"●" : @"○", name];
            NSMenuItem *accItem = [[NSMenuItem alloc] initWithTitle:title action:@selector(actionToggleAccount:) keyEquivalent:@""];
            [accItem setTarget:self];
            [accItem setTag:i];
            [accItem setState:enabled ? NSControlStateValueOn : NSControlStateValueOff];
            [menu addItem:accItem];
        }
    }
    [menu addItem:[NSMenuItem separatorItem]];

    // 4. Acciones de Configuración
    NSMenuItem *linkItem = [[NSMenuItem alloc] initWithTitle:@"+ Vincular Cuenta..." action:@selector(actionLinkAccount:) keyEquivalent:@""];
    [linkItem setTarget:self];
    [menu addItem:linkItem];

    NSMenuItem *cfgItem = [[NSMenuItem alloc] initWithTitle:@"⚙️ Abrir Configuración..." action:@selector(actionOpenConfig:) keyEquivalent:@""];
    [cfgItem setTarget:self];
    [menu addItem:cfgItem];
    [menu addItem:[NSMenuItem separatorItem]];

    // 5. Salir
    NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"❌ Salir" action:@selector(actionQuit:) keyEquivalent:@"q"];
    [quitItem setTarget:self];
    [menu addItem:quitItem];

    self.statusItem.menu = menu;
}

@end

void initMacTray(const char* iconPath) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        g_trayHandler = [[MacTrayHandler alloc] init];
        g_trayHandler.iconPath = [NSString stringWithUTF8String:iconPath];

        NSStatusBar *statusBar = [NSStatusBar systemStatusBar];
        g_trayHandler.statusItem = [statusBar statusItemWithLength:NSSquareStatusItemLength];

        NSImage *img = [[NSImage alloc] initWithContentsOfFile:g_trayHandler.iconPath];
        if (img) {
            [img setSize:NSMakeSize(18, 18)];
            g_trayHandler.statusItem.button.image = img;
        } else {
            g_trayHandler.statusItem.button.title = @"CF";
        }
    }
}

void setMacMenu(int masterActive, const char* accountsJson) {
    @autoreleasepool {
        NSString *jsonStr = [NSString stringWithUTF8String:accountsJson];
        NSData *data = [jsonStr dataUsingEncoding:NSUTF8StringEncoding];
        NSArray *accs = @[];
        if (data) {
            accs = [NSJSONSerialization JSONObjectWithData:data options:0 error:nil] ?: @[];
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            [g_trayHandler setupMenuWithMaster:(masterActive != 0) accounts:accs];
        });
    }
}

void runMacLoop(void) {
    @autoreleasepool {
        [NSApp run];
    }
}

void stopMacLoop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0) modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:YES];
    });
}
