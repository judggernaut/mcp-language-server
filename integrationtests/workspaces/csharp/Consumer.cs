namespace Workspace;

public static class Consumer
{
    // ConsumerFunction uses the helper function
    public static void ConsumerFunction()
    {
        var message = Helper.HelperFunction();
        Console.WriteLine(message);

        // Use shared class
        var s = new SharedClass
        {
            Id = 1,
            Name = "test",
            Value = 42.0,
            Constants = new List<string> { SharedConstants.SharedConstant },
        };

        // Call methods on the class
        Console.WriteLine(s.Method());
        s.Process();

        // Use shared interface
        SharedInterface iface = s;
        Console.WriteLine(iface.GetName());

        // Use shared type
        SharedType t = SharedType.First;
        Console.WriteLine(t);
    }
}
